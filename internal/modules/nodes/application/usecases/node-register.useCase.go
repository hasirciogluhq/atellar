package usecases

import (
	"context"
	"errors"
	"log"
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
	nodeRepository     ports.NodeRepositoryInterface
	overlayProvisioner ports.OverlayProvisioner
	peerNotifier       ports.PeerNotifier
}

func NewNodeRegisterUseCase(
	nodeRepository ports.NodeRepositoryInterface,
	overlayProvisioner ports.OverlayProvisioner,
	peerNotifier ports.PeerNotifier,
) *NodeRegisterUseCase {
	return &NodeRegisterUseCase{
		nodeRepository:     nodeRepository,
		overlayProvisioner: overlayProvisioner,
		peerNotifier:       peerNotifier,
	}
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

	if joinToken.SingleUse {
		if err := u.nodeRepository.MarkJoinTokenUsed(ctx, joinToken.ID, createdNode.ID); err != nil {
			return nil, err
		}
	}

	provisionedNode, err := u.overlayProvisioner.ProvisionNodeOverlay(ctx, createdNode.ID)
	if err != nil {
		return nil, err
	}

	apiKey, err := u.nodeRepository.IssueNodeAPIKey(ctx, createdNode.ID)
	if err != nil {
		return nil, err
	}

	if u.peerNotifier != nil {
		if err := u.peerNotifier.NotifyNodeAdded(ctx, *provisionedNode); err != nil {
			log.Printf("peer notification failed for node %s: %v", provisionedNode.ID, err)
		}
	}

	return &node.RegisterNodeResult{
		Node:            *provisionedNode,
		NodeAPIKey:      apiKey.NodeAPIKey,
		APIKeyExpiresAt: apiKey.APIKeyExpiresAt,
	}, nil
}
