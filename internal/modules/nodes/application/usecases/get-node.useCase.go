package usecases

import (
	"context"
	"errors"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type GetNodeUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewGetNodeUseCase(nodeRepository ports.NodeRepositoryInterface) *GetNodeUseCase {
	return &GetNodeUseCase{nodeRepository: nodeRepository}
}

func (u *GetNodeUseCase) Execute(ctx context.Context, nodeID string) (*node.NodeEntity, error) {
	foundNode, err := u.nodeRepository.GetNodeById(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	if foundNode == nil {
		return nil, errors.New("node not found")
	}

	return foundNode, nil
}
