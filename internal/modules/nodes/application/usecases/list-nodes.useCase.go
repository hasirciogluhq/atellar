package usecases

import (
	"context"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type ListNodesUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewListNodesUseCase(nodeRepository ports.NodeRepositoryInterface) *ListNodesUseCase {
	return &ListNodesUseCase{nodeRepository: nodeRepository}
}

func (u *ListNodesUseCase) Execute(ctx context.Context) ([]node.NodeEntity, error) {
	return u.nodeRepository.ListNodes(ctx)
}
