package usecases

import (
	"context"

	joinToken "github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/join-token"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type ListJoinTokensUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewListJoinTokensUseCase(nodeRepository ports.NodeRepositoryInterface) *ListJoinTokensUseCase {
	return &ListJoinTokensUseCase{nodeRepository: nodeRepository}
}

func (u *ListJoinTokensUseCase) Execute(ctx context.Context) ([]joinToken.JoinTokenEntity, error) {
	joinTokens, err := u.nodeRepository.ListJoinTokens(ctx)
	if err != nil {
		return nil, err
	}

	return joinTokens, nil
}
