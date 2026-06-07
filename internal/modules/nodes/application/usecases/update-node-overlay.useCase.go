package usecases

import (
	"context"
	"errors"
	"log"
	"net"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type UpdateNodeOverlayInput struct {
	NodeID      string
	OverlayIP   net.IP
	SubnetCIDR  string
}

type UpdateNodeOverlayUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
	peerNotifier   ports.PeerNotifier
}

func NewUpdateNodeOverlayUseCase(
	nodeRepository ports.NodeRepositoryInterface,
	peerNotifier ports.PeerNotifier,
) *UpdateNodeOverlayUseCase {
	return &UpdateNodeOverlayUseCase{
		nodeRepository: nodeRepository,
		peerNotifier:   peerNotifier,
	}
}

func (u *UpdateNodeOverlayUseCase) Execute(ctx context.Context, input UpdateNodeOverlayInput) (*node.NodeEntity, error) {
	if input.NodeID == "" {
		return nil, errors.New("node id is required")
	}

	if input.OverlayIP == nil || input.SubnetCIDR == "" {
		return nil, errors.New("overlay ip and subnet are required")
	}

	existing, err := u.nodeRepository.GetNodeById(ctx, input.NodeID)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return nil, errors.New("node not found")
	}

	previousOverlayIP := existing.OverlayIP.String()
	previousOverlaySubnet := existing.OverlaySubnet

	updatedNode, err := u.nodeRepository.UpdateNodeOverlayNetwork(
		ctx,
		input.NodeID,
		input.OverlayIP,
		input.SubnetCIDR,
		existing.Status,
	)
	if err != nil {
		return nil, err
	}

	overlayChanged := previousOverlayIP != updatedNode.OverlayIP.String() ||
		previousOverlaySubnet != updatedNode.OverlaySubnet

	if overlayChanged && u.peerNotifier != nil {
		if err := u.peerNotifier.NotifyNodeUpdated(ctx, *updatedNode, previousOverlayIP, previousOverlaySubnet); err != nil {
			log.Printf("peer notification failed for node overlay update %s: %v", updatedNode.ID, err)
		}
	}

	return updatedNode, nil
}
