package overlaynet

import "net"

const (
	DefaultBridgeName        = "atellar0"
	DefaultReconcileInterval = "30s"
)

type LocalNode struct {
	NodeID        string
	BridgeName    string
	OverlayIP     net.IP
	OverlaySubnet *net.IPNet
}

type PeerNode struct {
	NodeID        string
	OverlayIP     net.IP
	OverlaySubnet *net.IPNet
}

type RemoteContainer struct {
	ContainerID string
	NodeID      string
	OverlayIP   net.IP
	Via         net.IP
}

type RouteSpec struct {
	Destination *net.IPNet
	Via         net.IP
	Device      string
}

type PeerEvent struct {
	Event                 string
	NodeID                string
	OverlayIP             string
	OverlaySubnet         string
	PreviousOverlayIP     string
	PreviousOverlaySubnet string
	ContainerID           string
	Status                string
}
