package overlaynet

import (
	"context"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

// ClusterSyncer fetches cluster network state from the control plane over gRPC.
type ClusterSyncer interface {
	SyncClusterState(ctx context.Context) ([]node.NodeEntity, []container.Entity, error)
}
