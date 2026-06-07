package ports

import (
	"context"
	"net"
	"time"

	joinToken "github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/join-token"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

type NodeRepositoryInterface interface {
	UpdateNodeHeartbeat(ctx context.Context, nodeID string) error
	GetNodeById(ctx context.Context, nodeID string) (*node.NodeEntity, error)
	GetNodeByName(ctx context.Context, nodeName string) (*node.NodeEntity, error)
	CreateNode(ctx context.Context, name string, publicIP, privateIP net.IP) (*node.NodeEntity, error)
	CreateJoinToken(ctx context.Context, expiresAt *time.Time) (*joinToken.JoinTokenEntity, error)
	GetJoinToken(ctx context.Context, token string) (*joinToken.JoinTokenEntity, error)
	ListJoinTokens(ctx context.Context) ([]joinToken.JoinTokenEntity, error)
}
