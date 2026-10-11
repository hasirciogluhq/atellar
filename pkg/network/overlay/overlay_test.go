package overlay

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

// fake is an in-memory Dataplane that records every mutating call.
type fake struct {
	bridge    *Bridge
	vxlan     *VXLAN
	routes    map[string]Route
	neighbors map[string]Neighbor
	fdb       map[string]FDBEntry
	calls     []string
	failList  bool
}

func newFake() *fake {
	return &fake{routes: map[string]Route{}, neighbors: map[string]Neighbor{}, fdb: map[string]FDBEntry{}}
}

func (f *fake) record(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *fake) EnsureBridge(b Bridge) error { f.bridge = &b; return nil }
func (f *fake) EnsureVXLAN(v VXLAN) error   { f.vxlan = &v; return nil }

func (f *fake) ListRoutes(string) ([]Route, error) {
	if f.failList {
		return nil, errors.New("list failed")
	}
	return values(f.routes), nil
}
func (f *fake) ReplaceRoute(_ string, r Route) error {
	f.record("route replace %s via %s", r.Dst, r.Via)
	f.routes[r.Dst.String()] = r
	return nil
}
func (f *fake) DeleteRoute(_ string, r Route) error {
	f.record("route del %s", r.Dst)
	delete(f.routes, r.Dst.String())
	return nil
}

func (f *fake) ListNeighbors(string) ([]Neighbor, error) { return values(f.neighbors), nil }
func (f *fake) ReplaceNeighbor(_ string, n Neighbor) error {
	f.record("neigh replace %s %s", n.IP, n.MAC)
	f.neighbors[n.IP.String()] = n
	return nil
}
func (f *fake) DeleteNeighbor(_ string, n Neighbor) error {
	f.record("neigh del %s", n.IP)
	delete(f.neighbors, n.IP.String())
	return nil
}

func (f *fake) ListFDB(string) ([]FDBEntry, error) { return values(f.fdb), nil }
func (f *fake) ReplaceFDB(_ string, e FDBEntry) error {
	f.record("fdb replace %s %s", e.MAC, e.Dst)
	f.fdb[e.MAC.String()] = e
	return nil
}
func (f *fake) DeleteFDB(_ string, e FDBEntry) error {
	f.record("fdb del %s", e.MAC)
	delete(f.fdb, e.MAC.String())
	return nil
}

func values[T any](m map[string]T) []T {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(m))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

func node(id, subnet, overlayIP, privateIP string, status v1.NodeStatus) v1.Node {
	return v1.Node{ID: id, OverlaySubnet: subnet, OverlayIP: overlayIP, PrivateIP: privateIP, Status: status}
}

func TestVTEP(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("10.0.3.0/24")
	if got := VTEPAddr(subnet).String(); got != "10.0.3.0" {
		t.Errorf("VTEPAddr = %s", got)
	}
	if got := VTEPMAC(subnet).String(); got != "02:41:0a:00:03:00" {
		t.Errorf("VTEPMAC = %s", got)
	}
	_, other, _ := net.ParseCIDR("10.0.4.0/24")
	if VTEPMAC(subnet).String() == VTEPMAC(other).String() {
		t.Error("different subnets share a MAC")
	}
	if VTEPMAC(subnet)[0]&0x03 != 0x02 {
		t.Error("MAC is not locally administered unicast")
	}
}

// TestDesiredPeerMatrix checks which peers get entries for every node status
// and every combination of missing addressing fields.
func TestDesiredPeerMatrix(t *testing.T) {
	local := node("node_a", "10.0.1.0/24", "10.0.1.1", "192.168.0.1", v1.NodeStatusReady)
	statuses := []v1.NodeStatus{
		v1.NodeStatusPending, v1.NodeStatusReady, v1.NodeStatusNotReady, v1.NodeStatusMaintenance,
		v1.NodeStatusEvicting, v1.NodeStatusEvicted, v1.NodeStatusDown,
	}
	fields := []struct {
		name               string
		subnet, private    string
		public             string
		wantUnderlay       string
		routableIfStatusOK bool
	}{
		{"complete", "10.0.2.0/24", "192.168.0.2", "", "192.168.0.2", true},
		{"public only", "10.0.2.0/24", "", "203.0.113.2", "203.0.113.2", true},
		{"private preferred", "10.0.2.0/24", "192.168.0.2", "203.0.113.2", "192.168.0.2", true},
		{"no subnet", "", "192.168.0.2", "", "", false},
		{"bad subnet", "10.0.2.0/99", "192.168.0.2", "", "", false},
		{"no underlay", "10.0.2.0/24", "", "", "", false},
		{"ipv6 underlay", "10.0.2.0/24", "fd00::2", "", "", false},
		{"overlaps local", "10.0.1.0/24", "192.168.0.2", "", "", false},
		{"contains local", "10.0.0.0/16", "192.168.0.2", "", "", false},
	}
	for _, status := range statuses {
		for _, fc := range fields {
			t.Run(string(status)+"/"+fc.name, func(t *testing.T) {
				peer := node("node_b", fc.subnet, "", fc.private, status)
				peer.PublicIP = fc.public
				st, err := Desired(Config{NodeID: "node_a"}, []v1.Node{local, peer})
				if err != nil {
					t.Fatal(err)
				}
				want := fc.routableIfStatusOK && Routable(status)
				if got := len(st.Routes) == 1; got != want {
					t.Fatalf("peer routed = %v, want %v", got, want)
				}
				if len(st.Routes) != len(st.Neighbors) || len(st.Routes) != len(st.FDB) {
					t.Fatalf("tables disagree: %d routes, %d neighbors, %d fdb", len(st.Routes), len(st.Neighbors), len(st.FDB))
				}
				if !want {
					return
				}
				r, n, e := st.Routes[0], st.Neighbors[0], st.FDB[0]
				if r.Dst.String() != "10.0.2.0/24" || !r.Via.Equal(net.ParseIP("10.0.2.0")) {
					t.Errorf("route %s via %s", r.Dst, r.Via)
				}
				if !n.IP.Equal(r.Via) || n.MAC.String() != "02:41:0a:00:02:00" {
					t.Errorf("neighbor %s %s", n.IP, n.MAC)
				}
				if e.MAC.String() != n.MAC.String() || e.Dst.String() != fc.wantUnderlay {
					t.Errorf("fdb %s dst %s, want dst %s", e.MAC, e.Dst, fc.wantUnderlay)
				}
			})
		}
	}
}

func TestDesiredLocal(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		local  v1.Node
		ready  bool
		bridge string
		vtep   string
		under  string
	}{
		{"ready", Config{NodeID: "node_a"}, node("node_a", "10.0.1.0/24", "10.0.1.1", "192.168.0.1", v1.NodeStatusReady), true, "10.0.1.1/24", "10.0.1.0/32", "192.168.0.1"},
		{"underlay override", Config{NodeID: "node_a", UnderlayIP: "172.16.0.9"}, node("node_a", "10.0.1.0/24", "10.0.1.1", "192.168.0.1", v1.NodeStatusReady), true, "10.0.1.1/24", "10.0.1.0/32", "172.16.0.9"},
		{"no subnet", Config{NodeID: "node_a"}, node("node_a", "", "", "192.168.0.1", v1.NodeStatusPending), false, "", "", ""},
		{"no overlay ip", Config{NodeID: "node_a"}, node("node_a", "10.0.1.0/24", "", "192.168.0.1", v1.NodeStatusReady), false, "", "", ""},
		{"no underlay", Config{NodeID: "node_a"}, node("node_a", "10.0.1.0/24", "10.0.1.1", "", v1.NodeStatusReady), false, "", "", ""},
		{"not in list", Config{NodeID: "node_x"}, node("node_a", "10.0.1.0/24", "10.0.1.1", "192.168.0.1", v1.NodeStatusReady), false, "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := Desired(tc.cfg, []v1.Node{tc.local})
			if !tc.ready {
				if !errors.Is(err, ErrNotReady) {
					t.Fatalf("err = %v, want ErrNotReady", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if st.Bridge.Addr.String() != tc.bridge || st.VXLAN.Addr.String() != tc.vtep || st.VXLAN.Underlay.String() != tc.under {
				t.Fatalf("bridge %s vtep %s underlay %s", st.Bridge.Addr, st.VXLAN.Addr, st.VXLAN.Underlay)
			}
			if st.VXLAN.MTU != DefaultMTU || st.Bridge.MTU != DefaultMTU || st.VXLAN.VNI != DefaultVNI || st.VXLAN.Port != DefaultPort {
				t.Fatalf("defaults not applied: %+v", st.VXLAN)
			}
		})
	}
}

func cluster(peers int) []v1.Node {
	nodes := []v1.Node{node("node_a", "10.0.1.0/24", "10.0.1.1", "192.168.0.1", v1.NodeStatusReady)}
	for i := 2; i < peers+2; i++ {
		nodes = append(nodes, node(fmt.Sprintf("node_%d", i), fmt.Sprintf("10.0.%d.0/24", i), fmt.Sprintf("10.0.%d.1", i), fmt.Sprintf("192.168.0.%d", i), v1.NodeStatusReady))
	}
	return nodes
}

// TestReconcileLifecycle drives one manager through join, steady state,
// underlay change, leave and outside drift.
func TestReconcileLifecycle(t *testing.T) {
	dp := newFake()
	m, err := NewManager(Config{NodeID: "node_a"}, dp)
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Reconcile(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("empty node list: %v", err)
	}

	nodes := cluster(3)
	m.SetNodes(nodes)
	if err := m.Reconcile(); err != nil {
		t.Fatal(err)
	}
	if len(dp.routes) != 3 || len(dp.neighbors) != 3 || len(dp.fdb) != 3 || len(dp.calls) != 9 {
		t.Fatalf("join: %d routes %d neighbors %d fdb, %d calls", len(dp.routes), len(dp.neighbors), len(dp.fdb), len(dp.calls))
	}
	if dp.vxlan.MAC.String() != "02:41:0a:00:01:00" || dp.bridge.Addr.String() != "10.0.1.1/24" {
		t.Fatalf("local devices: %+v %+v", dp.vxlan, dp.bridge)
	}
	if !strings.HasPrefix(dp.calls[0], "fdb") || !strings.HasPrefix(dp.calls[8], "route") {
		t.Fatalf("fdb and neighbors must precede routes: %v", dp.calls)
	}

	dp.calls = nil
	if err := m.Reconcile(); err != nil || len(dp.calls) != 0 {
		t.Fatalf("steady state made changes: %v %v", dp.calls, err)
	}

	nodes[1].PrivateIP = "192.168.9.2"
	m.SetNodes(nodes)
	_ = m.Reconcile()
	if len(dp.calls) != 1 || dp.calls[0] != "fdb replace 02:41:0a:00:02:00 192.168.9.2" {
		t.Fatalf("underlay change: %v", dp.calls)
	}

	dp.calls = nil
	nodes[2].Status = v1.NodeStatusEvicted
	m.SetNodes(nodes)
	_ = m.Reconcile()
	if len(dp.calls) != 3 || len(dp.routes) != 2 {
		t.Fatalf("leave: %v", dp.calls)
	}
	for _, c := range dp.calls {
		if !strings.Contains(c, " del ") {
			t.Fatalf("leave issued %q", c)
		}
	}

	// Someone deleted a route and added a foreign one by hand.
	dp.calls = nil
	delete(dp.routes, "10.0.2.0/24")
	_, stray, _ := net.ParseCIDR("10.0.77.0/24")
	dp.routes[stray.String()] = Route{Dst: stray, Via: net.ParseIP("10.0.77.0")}
	_ = m.Reconcile()
	if len(dp.calls) != 2 || len(dp.routes) != 2 {
		t.Fatalf("drift repair: %v", dp.calls)
	}
	if _, ok := dp.routes[stray.String()]; ok {
		t.Fatal("stray route survived")
	}
}

func TestReconcileReportsListErrors(t *testing.T) {
	dp := newFake()
	dp.failList = true
	m, _ := NewManager(Config{NodeID: "node_a"}, dp)
	m.SetNodes(cluster(2))
	if err := m.Reconcile(); err == nil {
		t.Fatal("list failure was swallowed")
	}
	if len(dp.fdb) != 2 || len(dp.neighbors) != 2 {
		t.Fatal("a failing table must not block the others")
	}
}

func TestNewManagerValidation(t *testing.T) {
	if _, err := NewManager(Config{}, newFake()); err == nil {
		t.Error("missing node id accepted")
	}
	if _, err := NewManager(Config{NodeID: "n", UnderlayIP: "fd00::1"}, newFake()); err == nil {
		t.Error("IPv6 underlay accepted")
	}
}

func TestParseIproute2JSON(t *testing.T) {
	routes, err := parseRoutes([]byte(`[{"dst":"10.0.2.0/24","gateway":"10.0.2.0","flags":["onlink"]},{"dst":"10.0.9.9","gateway":"10.0.9.0","flags":["onlink"]},{"dst":"10.0.1.0/24","protocol":"kernel","scope":"link","prefsrc":"10.0.1.1","flags":[]}]`))
	if err != nil || len(routes) != 2 || routes[0].Dst.String() != "10.0.2.0/24" || routes[1].Dst.String() != "10.0.9.9/32" {
		t.Fatalf("routes = %+v, %v", routes, err)
	}
	neighbors, err := parseNeighbors([]byte(`[{"dst":"10.0.2.0","lladdr":"02:41:0a:00:02:00","state":["PERMANENT"]},{"dst":"10.0.3.0","state":["FAILED"]}]`))
	if err != nil || len(neighbors) != 1 || neighbors[0].MAC.String() != "02:41:0a:00:02:00" {
		t.Fatalf("neighbors = %+v, %v", neighbors, err)
	}
	fdb, err := parseFDB([]byte(`[{"mac":"02:41:0a:00:02:00","dst":"192.168.0.2","flags":["self"],"state":"permanent"},{"mac":"ae:12:00:00:00:01","flags":["self"],"state":"permanent"}]`))
	if err != nil || len(fdb) != 1 || fdb[0].Dst.String() != "192.168.0.2" {
		t.Fatalf("fdb = %+v, %v", fdb, err)
	}
	for _, empty := range []string{"", "  \n", "[]"} {
		if r, err := parseRoutes([]byte(empty)); err != nil || len(r) != 0 {
			t.Errorf("parseRoutes(%q) = %v, %v", empty, r, err)
		}
	}
	if _, err := parseFDB([]byte("{")); err == nil {
		t.Error("bad json accepted")
	}
}
