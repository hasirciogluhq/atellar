package usecases

import (
	"context"
	"errors"

	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/nodes/ports"
)

type NodeHeartbeatUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewNodeHeartbeatUseCase(infra *shared.Infrastructure) *NodeHeartbeatUseCase {
	return &NodeHeartbeatUseCase{nodeRepository: infra.Repositories.Nodes}
}

func (u *NodeHeartbeatUseCase) IDNodeResolution(ctx context.Context, nodeID string) (*node.NodeEntity, error) {
	return u.nodeRepository.GetNodeById(ctx, nodeID)
}

func (u *NodeHeartbeatUseCase) NameNodeResolution(ctx context.Context, nodeName string) (*node.NodeEntity, error) {
	return u.nodeRepository.GetNodeByName(ctx, nodeName)
}

func (u *NodeHeartbeatUseCase) NodeResolution(ctx context.Context, identifier string) (*node.NodeEntity, error) {
	identifierType, err := node.ResolveNodeResolutionIdentifierContext(identifier)
	if err != nil {
		return nil, err
	}

	switch identifierType {
	case node.NodeResolutionIdentifierTypeID:
		return u.IDNodeResolution(ctx, identifier)
	case node.NodeResolutionIdentifierTypeName:
		return u.NameNodeResolution(ctx, identifier)
	default:
		return nil, errors.New("invalid node resolution identifier")
	}
}

func (u *NodeHeartbeatUseCase) Execute(ctx context.Context, nodeID string) error {
	node, err := u.NodeResolution(ctx, nodeID)
	if err != nil {
		return err
	}

	if node == nil {
		return errors.New("node not found by identifier: " + nodeID)
	}

	return u.nodeRepository.UpdateNodeHeartbeat(ctx, node.ID)
}
