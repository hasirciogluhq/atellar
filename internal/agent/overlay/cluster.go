package overlay

import (
	"context"
	"net"
)

const (
	nodeStatusEvicted   = "evicted"
	nodeStatusEvicting  = "evicting"
	nodeStatusDown      = "down"
	containerStatusScheduled = "scheduled"
	containerStatusPending   = "pending"
	containerStatusPulling   = "pulling"
	containerStatusCreating  = "creating"
	containerStatusRunning   = "running"
)

// ClusterNode is a minimal node view for overlay routing (no domain coupling).
type ClusterNode struct {
	ID            string
	OverlayIP     net.IP
	OverlaySubnet string
	Status        string
}

// ClusterContainer is a minimal container view for overlay routing.
type ClusterContainer struct {
	ID        string
	NodeID    string
	OverlayIP net.IP
	Status    string
}

// ClusterSyncer loads cluster overlay state from the control plane (gRPC).
type ClusterSyncer interface {
	SyncClusterState(ctx context.Context) ([]ClusterNode, []ClusterContainer, error)
}
