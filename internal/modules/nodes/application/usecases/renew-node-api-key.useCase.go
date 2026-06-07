package usecases

import (
	"context"
	"errors"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type RenewNodeAPIKeyUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewRenewNodeAPIKeyUseCase(nodeRepository ports.NodeRepositoryInterface) *RenewNodeAPIKeyUseCase {
	return &RenewNodeAPIKeyUseCase{nodeRepository: nodeRepository}
}

func (u *RenewNodeAPIKeyUseCase) Execute(ctx context.Context, plainAPIKey string) (*node.NodeAPIKeyResult, error) {
	if plainAPIKey == "" {
		return nil, errors.New("node api key is required")
	}

	result, err := u.nodeRepository.RenewNodeAPIKey(ctx, plainAPIKey)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, errors.New("failed to renew node api key")
	}

	return result, nil
}
