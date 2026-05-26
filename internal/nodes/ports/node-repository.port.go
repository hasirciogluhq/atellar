package ports

import (
	"context"

	"github.com/hasirciogluhq/atellar/internal/nodes/domain/node"
)

type NodeRepositoryInterface interface {
	UpdateNodeHeartbeat(ctx context.Context, nodeID string) error
	GetNodeById(ctx context.Context, nodeID string) (*node.NodeEntity, error)
	GetNodeByName(ctx context.Context, nodeName string) (*node.NodeEntity, error)
}
