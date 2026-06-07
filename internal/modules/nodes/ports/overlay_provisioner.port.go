package ports

import (
	"context"
	"net"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

type OverlayProvisioner interface {
	ProvisionNodeOverlay(ctx context.Context, nodeID string) (*node.NodeEntity, error)
}

type OverlayNetworkInput struct {
	NodeID     string
	NodeIP     net.IP
	SubnetCIDR string
	PoolIPs    []net.IP
}
