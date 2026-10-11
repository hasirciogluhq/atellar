package overlay

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// Route sends a peer subnet to the peer's VTEP address over the VXLAN
// device. Onlink is implied: the VTEP address is not on a local subnet.
type Route struct {
	Dst *net.IPNet
	Via net.IP
}

// Neighbor resolves a peer VTEP address to the peer's VXLAN MAC.
type Neighbor struct {
	IP  net.IP
	MAC net.HardwareAddr
}

// FDBEntry tells the VXLAN device which underlay address owns a MAC.
type FDBEntry struct {
	MAC net.HardwareAddr
	Dst net.IP
}

// VXLAN describes the node's VXLAN device.
type VXLAN struct {
	Name     string
	VNI      int
	Port     int
	Underlay net.IP           // local underlay address packets leave from
	MAC      net.HardwareAddr // derived from the node subnet
	Addr     *net.IPNet       // the node VTEP address as /32
	MTU      int
}

// Bridge describes the bridge containers attach to.
type Bridge struct {
	Name string
	Addr *net.IPNet // node overlay IP with the subnet mask
	MTU  int
}

// Dataplane programs the kernel. The Linux implementation drives iproute2;
// tests use an in-memory fake.
type Dataplane interface {
	EnsureBridge(Bridge) error
	EnsureVXLAN(VXLAN) error

	ListRoutes(dev string) ([]Route, error)
	ReplaceRoute(dev string, r Route) error
	DeleteRoute(dev string, r Route) error

	ListNeighbors(dev string) ([]Neighbor, error)
	ReplaceNeighbor(dev string, n Neighbor) error
	DeleteNeighbor(dev string, n Neighbor) error

	ListFDB(dev string) ([]FDBEntry, error)
	ReplaceFDB(dev string, e FDBEntry) error
	DeleteFDB(dev string, e FDBEntry) error
}

// VTEPAddr is the address a node's VXLAN device holds: the network address
// of its subnet (for 10.0.3.0/24 it is 10.0.3.0). Containers never use it,
// so it costs no pool address.
func VTEPAddr(subnet *net.IPNet) net.IP {
	return subnet.IP.Mask(subnet.Mask).To4()
}

// VTEPMAC derives a stable, locally administered MAC from a node subnet, so
// every node can compute every peer's MAC without asking it.
func VTEPMAC(subnet *net.IPNet) net.HardwareAddr {
	ip := VTEPAddr(subnet)
	return net.HardwareAddr{0x02, 0x41, ip[0], ip[1], ip[2], ip[3]}
}

// The parse functions below read iproute2 JSON output ("ip -j", "bridge -j").
// They are platform independent so they can be tested anywhere.

func parseRoutes(data []byte) ([]Route, error) {
	var rows []struct {
		Dst     string `json:"dst"`
		Gateway string `json:"gateway"`
	}
	if err := unmarshalRows(data, &rows); err != nil {
		return nil, fmt.Errorf("parse routes: %w", err)
	}
	out := make([]Route, 0, len(rows))
	for _, row := range rows {
		via := net.ParseIP(row.Gateway)
		if via == nil {
			continue // connected routes are not ours
		}
		dst := row.Dst
		if !strings.Contains(dst, "/") {
			dst += "/32"
		}
		_, n, err := net.ParseCIDR(dst)
		if err != nil {
			continue
		}
		out = append(out, Route{Dst: n, Via: via})
	}
	return out, nil
}

func parseNeighbors(data []byte) ([]Neighbor, error) {
	var rows []struct {
		Dst    string `json:"dst"`
		LLAddr string `json:"lladdr"`
	}
	if err := unmarshalRows(data, &rows); err != nil {
		return nil, fmt.Errorf("parse neighbors: %w", err)
	}
	out := make([]Neighbor, 0, len(rows))
	for _, row := range rows {
		ip := net.ParseIP(row.Dst)
		mac, err := net.ParseMAC(row.LLAddr)
		if ip == nil || err != nil {
			continue
		}
		out = append(out, Neighbor{IP: ip, MAC: mac})
	}
	return out, nil
}

func parseFDB(data []byte) ([]FDBEntry, error) {
	var rows []struct {
		MAC string `json:"mac"`
		Dst string `json:"dst"`
	}
	if err := unmarshalRows(data, &rows); err != nil {
		return nil, fmt.Errorf("parse fdb: %w", err)
	}
	out := make([]FDBEntry, 0, len(rows))
	for _, row := range rows {
		mac, err := net.ParseMAC(row.MAC)
		dst := net.ParseIP(row.Dst)
		if err != nil || dst == nil {
			continue // entries without a remote are the device's own
		}
		out = append(out, FDBEntry{MAC: mac, Dst: dst})
	}
	return out, nil
}

func unmarshalRows(data []byte, rows any) error {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, rows)
}
