// Package overlay builds the cluster network on each node.
//
// Every node owns one subnet of the cluster CIDR (assigned by the API
// server). Containers attach to a local bridge that holds the node's first
// subnet address. Traffic to another node's subnet goes through one VXLAN
// device per node, without per-container state:
//
//	route   <peer subnet> via <peer VTEP> dev <vxlan> onlink
//	neigh   <peer VTEP> lladdr <peer MAC> dev <vxlan> nud permanent
//	fdb     <peer MAC> dev <vxlan> dst <peer underlay IP>
//
// The VTEP address is the subnet's network address and the MAC is derived
// from the subnet, so every node computes all three entries from the node
// list alone. All nodes must reach each other's underlay address directly.
package overlay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

const (
	DefaultBridgeName = "atellar0"
	DefaultVXLANName  = "atellar-vx"
	DefaultVNI        = 4242
	DefaultPort       = 4789
	// DefaultMTU leaves room for the 50 byte VXLAN header on a 1500 byte
	// underlay.
	DefaultMTU               = 1450
	DefaultReconcileInterval = 30 * time.Second
)

type Config struct {
	// NodeID is the local node; its entry in the node list provides the
	// local subnet and, unless UnderlayIP is set, the underlay address.
	NodeID     string
	UnderlayIP string
	BridgeName string
	VXLANName  string
	VNI        int
	Port       int
	MTU        int
	// ReconcileInterval repairs drift made outside the agent.
	ReconcileInterval time.Duration
}

func (c *Config) setDefaults() {
	if c.BridgeName == "" {
		c.BridgeName = DefaultBridgeName
	}
	if c.VXLANName == "" {
		c.VXLANName = DefaultVXLANName
	}
	if c.VNI == 0 {
		c.VNI = DefaultVNI
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.MTU == 0 {
		c.MTU = DefaultMTU
	}
	if c.ReconcileInterval <= 0 {
		c.ReconcileInterval = DefaultReconcileInterval
	}
}

// ErrNotReady means the local node has no subnet or underlay address yet.
var ErrNotReady = errors.New("local node has no overlay subnet or underlay address yet")

// Manager keeps the kernel in line with the latest node list.
type Manager struct {
	cfg     Config
	dp      Dataplane
	trigger chan struct{}

	mu    sync.Mutex
	nodes []v1.Node

	reconcile sync.Mutex
}

func NewManager(cfg Config, dp Dataplane) (*Manager, error) {
	if cfg.NodeID == "" {
		return nil, errors.New("overlay: node id is required")
	}
	if cfg.UnderlayIP != "" && net.ParseIP(cfg.UnderlayIP).To4() == nil {
		return nil, fmt.Errorf("overlay: underlay ip %q is not IPv4", cfg.UnderlayIP)
	}
	cfg.setDefaults()
	if dp == nil {
		dp = NewDataplane()
	}
	return &Manager{cfg: cfg, dp: dp, trigger: make(chan struct{}, 1)}, nil
}

// Config returns the effective configuration.
func (m *Manager) Config() Config { return m.cfg }

// SetNodes replaces the node list and schedules a reconcile.
func (m *Manager) SetNodes(nodes []v1.Node) {
	m.mu.Lock()
	m.nodes = append(m.nodes[:0:0], nodes...)
	m.mu.Unlock()
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

// Run reconciles after every SetNodes and every ReconcileInterval until ctx
// is done.
func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-m.trigger:
		}
		if err := m.Reconcile(); err != nil && !errors.Is(err, ErrNotReady) {
			log.Printf("overlay: reconcile: %v", err)
		}
	}
}

// State is the full desired kernel state of one node.
type State struct {
	Bridge    Bridge
	VXLAN     VXLAN
	Routes    []Route
	Neighbors []Neighbor
	FDB       []FDBEntry
}

// Desired computes the kernel state for the local node from a node list.
func Desired(cfg Config, nodes []v1.Node) (*State, error) {
	cfg.setDefaults()
	var local *v1.Node
	for i := range nodes {
		if nodes[i].ID == cfg.NodeID {
			local = &nodes[i]
			break
		}
	}
	if local == nil {
		return nil, ErrNotReady
	}
	localSubnet := parseSubnet(local.OverlaySubnet)
	localIP := net.ParseIP(local.OverlayIP).To4()
	underlay := net.ParseIP(cfg.UnderlayIP).To4()
	if underlay == nil {
		underlay = underlayIP(*local)
	}
	if localSubnet == nil || localIP == nil || underlay == nil {
		return nil, ErrNotReady
	}

	st := &State{
		Bridge: Bridge{
			Name: cfg.BridgeName,
			Addr: &net.IPNet{IP: localIP, Mask: localSubnet.Mask},
			MTU:  cfg.MTU,
		},
		VXLAN: VXLAN{
			Name:     cfg.VXLANName,
			VNI:      cfg.VNI,
			Port:     cfg.Port,
			Underlay: underlay,
			MAC:      VTEPMAC(localSubnet),
			Addr:     &net.IPNet{IP: VTEPAddr(localSubnet), Mask: net.CIDRMask(32, 32)},
			MTU:      cfg.MTU,
		},
	}

	for _, n := range nodes {
		if n.ID == cfg.NodeID || !Routable(n.Status) {
			continue
		}
		subnet := parseSubnet(n.OverlaySubnet)
		peerUnderlay := underlayIP(n)
		if subnet == nil || peerUnderlay == nil || overlaps(subnet, localSubnet) {
			continue
		}
		vtep, mac := VTEPAddr(subnet), VTEPMAC(subnet)
		st.Routes = append(st.Routes, Route{Dst: subnet, Via: vtep})
		st.Neighbors = append(st.Neighbors, Neighbor{IP: vtep, MAC: mac})
		st.FDB = append(st.FDB, FDBEntry{MAC: mac, Dst: peerUnderlay})
	}
	sort.Slice(st.Routes, func(i, j int) bool { return st.Routes[i].Dst.String() < st.Routes[j].Dst.String() })
	sort.Slice(st.Neighbors, func(i, j int) bool { return st.Neighbors[i].IP.String() < st.Neighbors[j].IP.String() })
	sort.Slice(st.FDB, func(i, j int) bool { return st.FDB[i].MAC.String() < st.FDB[j].MAC.String() })
	return st, nil
}

// Routable reports whether peers keep a path to a node in this state. A
// node that missed heartbeats keeps its routes: its containers may still
// run and the failure may be on the control plane side only.
func Routable(s v1.NodeStatus) bool {
	switch s {
	case v1.NodeStatusEvicting, v1.NodeStatusEvicted, v1.NodeStatusDown:
		return false
	}
	return true
}

// underlayIP prefers the private address; nodes must share one underlay.
func underlayIP(n v1.Node) net.IP {
	for _, s := range []string{n.PrivateIP, n.PublicIP} {
		if ip := net.ParseIP(s).To4(); ip != nil {
			return ip
		}
	}
	return nil
}

func parseSubnet(s string) *net.IPNet {
	if s == "" {
		return nil
	}
	_, n, err := net.ParseCIDR(s)
	if err != nil || n.IP.To4() == nil {
		return nil
	}
	return n
}

func overlaps(a, b *net.IPNet) bool { return a.Contains(b.IP) || b.Contains(a.IP) }

// Reconcile applies the desired state: missing or changed entries are
// replaced and entries for nodes that left are deleted. Entries already
// correct are not touched, so a steady state costs three list calls.
func (m *Manager) Reconcile() error {
	m.reconcile.Lock()
	defer m.reconcile.Unlock()

	m.mu.Lock()
	nodes := m.nodes
	m.mu.Unlock()

	st, err := Desired(m.cfg, nodes)
	if err != nil {
		return err
	}
	if err := m.dp.EnsureBridge(st.Bridge); err != nil {
		return fmt.Errorf("bridge %s: %w", st.Bridge.Name, err)
	}
	if err := m.dp.EnsureVXLAN(st.VXLAN); err != nil {
		return fmt.Errorf("vxlan %s: %w", st.VXLAN.Name, err)
	}

	dev := st.VXLAN.Name
	var errs []error
	// FDB and neighbors first, so a new route never points at an
	// unresolvable next hop.
	errs = append(errs, sync3(dev, st.FDB, m.dp.ListFDB, m.dp.ReplaceFDB, m.dp.DeleteFDB,
		func(e FDBEntry) string { return e.MAC.String() },
		func(a, b FDBEntry) bool { return a.Dst.Equal(b.Dst) }))
	errs = append(errs, sync3(dev, st.Neighbors, m.dp.ListNeighbors, m.dp.ReplaceNeighbor, m.dp.DeleteNeighbor,
		func(n Neighbor) string { return n.IP.String() },
		func(a, b Neighbor) bool { return a.MAC.String() == b.MAC.String() }))
	errs = append(errs, sync3(dev, st.Routes, m.dp.ListRoutes, m.dp.ReplaceRoute, m.dp.DeleteRoute,
		func(r Route) string { return r.Dst.String() },
		func(a, b Route) bool { return a.Via.Equal(b.Via) }))
	return errors.Join(errs...)
}

// sync3 diffs one table: list current entries, replace missing or changed
// ones, delete the rest. key identifies an entry, same compares values.
func sync3[T any](
	dev string,
	desired []T,
	list func(string) ([]T, error),
	replace, del func(string, T) error,
	key func(T) string,
	same func(a, b T) bool,
) error {
	current, err := list(dev)
	if err != nil {
		return err
	}
	have := make(map[string]T, len(current))
	for _, c := range current {
		have[key(c)] = c
	}
	want := make(map[string]bool, len(desired))
	var errs []error
	for _, d := range desired {
		k := key(d)
		want[k] = true
		if c, ok := have[k]; ok && same(c, d) {
			continue
		}
		if err := replace(dev, d); err != nil {
			errs = append(errs, err)
		}
	}
	for k, c := range have {
		if !want[k] {
			if err := del(dev, c); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
