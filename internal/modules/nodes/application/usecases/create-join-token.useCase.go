package usecases

import (
	"context"
	"errors"
	"time"

	joinToken "github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/join-token"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type CreateJoinTokenUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewCreateJoinTokenUseCase(nodeRepository ports.NodeRepositoryInterface) *CreateJoinTokenUseCase {
	return &CreateJoinTokenUseCase{nodeRepository: nodeRepository}
}

func (u *CreateJoinTokenUseCase) Execute(ctx context.Context, expiresAt *time.Time, singleUse bool) (*joinToken.JoinTokenCreateResult, error) {
	createdToken, err := u.nodeRepository.CreateJoinToken(ctx, expiresAt, singleUse)
	if err != nil {
		return nil, err
	}

	if createdToken == nil {
		return nil, errors.New("failed to create join token")
	}

	return createdToken, nil
}
