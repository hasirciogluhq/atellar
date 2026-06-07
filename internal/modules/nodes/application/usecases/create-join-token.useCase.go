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

func (u *CreateJoinTokenUseCase) Execute(ctx context.Context, expiresAt *time.Time) (*joinToken.JoinTokenEntity, error) {
	joinToken, err := u.nodeRepository.CreateJoinToken(ctx, expiresAt)
	if err != nil {
		return nil, err
	}

	if joinToken == nil {
		return nil, errors.New("failed to create join token")
	}

	return joinToken, nil
}
