package usecases

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type RegisterNodeInput struct {
	Token          string
	Name           string
	PublicIP       net.IP
	PrivateIP      net.IP
	AgentVersion   *string
	ContainerdSock string
}

type NodeRegisterUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewNodeRegisterUseCase(nodeRepository ports.NodeRepositoryInterface) *NodeRegisterUseCase {
	return &NodeRegisterUseCase{nodeRepository: nodeRepository}
}

func (u *NodeRegisterUseCase) Execute(ctx context.Context, input RegisterNodeInput) (*node.RegisterNodeResult, error) {
	if input.Token == "" {
		return nil, errors.New("join token is required")
	}

	joinToken, err := u.nodeRepository.GetJoinToken(ctx, input.Token)
	if err != nil {
		return nil, err
	}

	if joinToken == nil {
		return nil, errors.New("invalid join token")
	}

	if joinToken.ExpiresAt != nil && joinToken.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("join token expired")
	}

	if joinToken.SingleUse && joinToken.UsedAt != nil {
		return nil, errors.New("join token already used")
	}

	createdNode, err := u.nodeRepository.CreateNode(ctx, ports.CreateNodeInput{
		Name:           input.Name,
		PublicIP:       input.PublicIP,
		PrivateIP:      input.PrivateIP,
		AgentVersion:   input.AgentVersion,
		ContainerdSock: input.ContainerdSock,
	})
	if err != nil {
		return nil, err
	}

	if createdNode == nil {
		return nil, errors.New("failed to register node")
	}

	if err := u.nodeRepository.MarkJoinTokenUsed(ctx, joinToken.ID, createdNode.ID); err != nil {
		return nil, err
	}

	apiKey, err := u.nodeRepository.IssueNodeAPIKey(ctx, createdNode.ID)
	if err != nil {
		return nil, err
	}

	return &node.RegisterNodeResult{
		Node:            *createdNode,
		NodeAPIKey:      apiKey.NodeAPIKey,
		APIKeyExpiresAt: apiKey.APIKeyExpiresAt,
	}, nil
}
