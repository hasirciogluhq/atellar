package services

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/cluster/ipam"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	nodeports "github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type OverlayProvisioner struct {
	nodeRepository      nodeports.NodeRepositoryInterface
	containerRepository ports.ContainerRepositoryInterface
	allocator           *ipam.Allocator
}

func NewOverlayProvisioner(
	nodeRepository nodeports.NodeRepositoryInterface,
	containerRepository ports.ContainerRepositoryInterface,
	clusterCIDR string,
	nodeSubnetPrefixLen int,
) (*OverlayProvisioner, error) {
	allocator, err := ipam.NewAllocator(clusterCIDR, nodeSubnetPrefixLen)
	if err != nil {
		return nil, err
	}

	return &OverlayProvisioner{
		nodeRepository:      nodeRepository,
		containerRepository: containerRepository,
		allocator:           allocator,
	}, nil
}

func (p *OverlayProvisioner) ProvisionNodeOverlay(ctx context.Context, nodeID string) (*node.NodeEntity, error) {
	reclaimable, err := p.nodeRepository.ListReclaimableOverlayNetworks(ctx)
	if err != nil {
		return nil, err
	}

	activeSubnets, err := p.nodeRepository.ListActiveNodeOverlaySubnets(ctx)
	if err != nil {
		return nil, err
	}

	reclaimCandidates := make([]ipam.ReclaimableSubnet, 0, len(reclaimable))
	for _, candidate := range reclaimable {
		reclaimCandidates = append(reclaimCandidates, ipam.ReclaimableSubnet{
			SourceNodeID: candidate.NodeID,
			SubnetCIDR:   candidate.SubnetCIDR,
		})
	}

	result, err := p.allocator.Allocate(reclaimCandidates, activeSubnets)
	if err != nil {
		return nil, err
	}

	if result.SourceNodeID != "" {
		if err := p.containerRepository.DeleteOverlayIPPoolByNodeId(ctx, result.SourceNodeID); err != nil {
			return nil, fmt.Errorf("clear reclaimed overlay pool: %w", err)
		}

		if err := p.nodeRepository.ClearNodeOverlayNetwork(ctx, result.SourceNodeID); err != nil {
			return nil, fmt.Errorf("clear reclaimed overlay network: %w", err)
		}
	}

	updatedNode, err := p.nodeRepository.UpdateNodeOverlayNetwork(
		ctx,
		nodeID,
		result.Allocation.NodeIP,
		result.Allocation.SubnetCIDR,
		node.NodeStatusReady,
	)
	if err != nil {
		return nil, err
	}

	for _, poolIP := range result.Allocation.PoolIPs {
		if _, err := p.containerRepository.CreateOverlayIPPoolEntry(ctx, poolIP, nodeID); err != nil {
			return nil, fmt.Errorf("seed overlay pool for %s: %w", poolIP, err)
		}
	}

	return updatedNode, nil
}
