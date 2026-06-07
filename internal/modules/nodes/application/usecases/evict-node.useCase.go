package usecases

import (
	"context"
	"errors"
	"log"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type EvictNodeUseCase struct {
	nodeRepository ports.NodeRepositoryInterface
	peerNotifier   ports.PeerNotifier
}

func NewEvictNodeUseCase(
	nodeRepository ports.NodeRepositoryInterface,
	peerNotifier ports.PeerNotifier,
) *EvictNodeUseCase {
	return &EvictNodeUseCase{
		nodeRepository: nodeRepository,
		peerNotifier:   peerNotifier,
	}
}

func (u *EvictNodeUseCase) Execute(ctx context.Context, nodeID string) (*node.NodeEntity, error) {
	if nodeID == "" {
		return nil, errors.New("node id is required")
	}

	existing, err := u.nodeRepository.GetNodeById(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		return nil, errors.New("node not found")
	}

	if existing.Status == node.NodeStatusEvicted {
		return existing, nil
	}

	evictedNode, err := u.nodeRepository.EvictNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	if u.peerNotifier != nil && evictedNode.OverlaySubnet != "" {
		if err := u.peerNotifier.NotifyNodeRemoved(ctx, *evictedNode); err != nil {
			log.Printf("peer notification failed for evicted node %s: %v", evictedNode.ID, err)
		}
	}

	return evictedNode, nil
}
