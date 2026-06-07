package usecases

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type NodeRegisterUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewNodeRegisterUseCase(nodeRepository ports.NodeRepositoryInterface) *NodeRegisterUseCase {
	return &NodeRegisterUseCase{nodeRepository: nodeRepository}
}

func (u *NodeRegisterUseCase) Execute(ctx context.Context, token, name string, publicIP, privateIP net.IP) (*node.NodeEntity, error) {
	if token == "" {
		return nil, errors.New("join token is required")
	}

	joinToken, err := u.nodeRepository.GetJoinToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if joinToken == nil {
		return nil, errors.New("invalid join token")
	}

	if joinToken.ExpiresAt != nil && joinToken.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("join token expired")
	}

	createdNode, err := u.nodeRepository.CreateNode(ctx, name, publicIP, privateIP)
	if err != nil {
		return nil, err
	}

	if createdNode == nil {
		return nil, errors.New("failed to register node")
	}

	return createdNode, nil
}
