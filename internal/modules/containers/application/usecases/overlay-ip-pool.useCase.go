package usecases

import (
	"context"
	"errors"
	"net"

	overlayippool "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/overlay-ip-pool"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type CreateOverlayIPPoolEntryUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewCreateOverlayIPPoolEntryUseCase(containerRepository ports.ContainerRepositoryInterface) *CreateOverlayIPPoolEntryUseCase {
	return &CreateOverlayIPPoolEntryUseCase{containerRepository: containerRepository}
}

func (u *CreateOverlayIPPoolEntryUseCase) Execute(ctx context.Context, ip net.IP, nodeID string) (*overlayippool.Entity, error) {
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}

	if ip == nil {
		return nil, errors.New("ip is required")
	}

	return u.containerRepository.CreateOverlayIPPoolEntry(ctx, ip, nodeID)
}

type ListOverlayIPsUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewListOverlayIPsUseCase(containerRepository ports.ContainerRepositoryInterface) *ListOverlayIPsUseCase {
	return &ListOverlayIPsUseCase{containerRepository: containerRepository}
}

func (u *ListOverlayIPsUseCase) Execute(ctx context.Context, nodeID string, freeOnly bool) ([]overlayippool.Entity, error) {
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}

	if freeOnly {
		return u.containerRepository.ListFreeOverlayIPsByNodeId(ctx, nodeID)
	}

	return u.containerRepository.ListOverlayIPsByNodeId(ctx, nodeID)
}

type AllocateOverlayIPUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewAllocateOverlayIPUseCase(containerRepository ports.ContainerRepositoryInterface) *AllocateOverlayIPUseCase {
	return &AllocateOverlayIPUseCase{containerRepository: containerRepository}
}

func (u *AllocateOverlayIPUseCase) Execute(ctx context.Context, ip net.IP, containerID string) (*overlayippool.Entity, error) {
	if containerID == "" {
		return nil, errors.New("container_id is required")
	}

	if ip == nil {
		return nil, errors.New("ip is required")
	}

	allocated, err := u.containerRepository.AllocateOverlayIP(ctx, ip, containerID)
	if err != nil {
		return nil, err
	}

	if allocated == nil {
		return nil, errors.New("overlay ip not available")
	}

	return allocated, nil
}

type ReleaseOverlayIPUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewReleaseOverlayIPUseCase(containerRepository ports.ContainerRepositoryInterface) *ReleaseOverlayIPUseCase {
	return &ReleaseOverlayIPUseCase{containerRepository: containerRepository}
}

func (u *ReleaseOverlayIPUseCase) Execute(ctx context.Context, ip net.IP) error {
	if ip == nil {
		return errors.New("ip is required")
	}

	return u.containerRepository.ReleaseOverlayIP(ctx, ip)
}
