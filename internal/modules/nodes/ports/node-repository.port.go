package ports

import (
	"context"
	"net"
	"time"

	joinToken "github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/join-token"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

type CreateNodeInput struct {
	Name           string
	PublicIP       net.IP
	PrivateIP      net.IP
	AgentVersion   *string
	ContainerdSock string
}

type NodeRepositoryInterface interface {
	UpdateNodeHeartbeat(ctx context.Context, nodeID string) error
	GetNodeById(ctx context.Context, nodeID string) (*node.NodeEntity, error)
	GetNodeByName(ctx context.Context, nodeName string) (*node.NodeEntity, error)
	ListNodes(ctx context.Context) ([]node.NodeEntity, error)
	CreateNode(ctx context.Context, input CreateNodeInput) (*node.NodeEntity, error)
	IssueNodeAPIKey(ctx context.Context, nodeID string) (*node.NodeAPIKeyResult, error)
	AuthenticateNodeByAPIKey(ctx context.Context, plainAPIKey string) (*node.NodeEntity, error)
	RenewNodeAPIKey(ctx context.Context, plainAPIKey string) (*node.NodeAPIKeyResult, error)
	CreateJoinToken(ctx context.Context, expiresAt *time.Time, singleUse bool) (*joinToken.JoinTokenCreateResult, error)
	GetJoinToken(ctx context.Context, plainToken string) (*joinToken.JoinTokenEntity, error)
	ListJoinTokens(ctx context.Context) ([]joinToken.JoinTokenEntity, error)
	MarkJoinTokenUsed(ctx context.Context, tokenID, nodeID string) error
}
