package usecases

import (
	"context"
	"errors"
	"net"

	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type AllocateContainerOverlayIPUseCase struct {
	containers ports.ContainerRepositoryInterface
}

func NewAllocateContainerOverlayIPUseCase(containers ports.ContainerRepositoryInterface) *AllocateContainerOverlayIPUseCase {
	return &AllocateContainerOverlayIPUseCase{containers: containers}
}

func (u *AllocateContainerOverlayIPUseCase) Execute(ctx context.Context, nodeID, containerID string) (net.IP, error) {
	found, err := u.containers.GetContainerById(ctx, containerID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("container not found")
	}
	if found.NodeID != nodeID {
		return nil, errors.New("container does not belong to this node")
	}
	if found.OverlayIP != nil {
		return found.OverlayIP, nil
	}

	allocated, err := u.containers.AllocateFirstFreeOverlayIP(ctx, nodeID, containerID)
	if err != nil {
		return nil, err
	}
	if allocated == nil {
		return nil, errors.New("no free overlay ip on node")
	}

	updated, err := u.containers.UpdateContainerRuntime(ctx, containerID, ports.UpdateContainerRuntimeInput{
		OverlayIP: allocated.IP,
		Status:    found.Status,
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("failed to update container overlay ip")
	}

	return allocated.IP, nil
}
