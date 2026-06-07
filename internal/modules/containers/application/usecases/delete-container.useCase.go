package usecases

import (
	"context"
	"errors"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type DeleteContainerUseCase struct {
	containers ports.ContainerRepositoryInterface
	peerNotify ports.PeerNotifier
}

func NewDeleteContainerUseCase(containers ports.ContainerRepositoryInterface, peerNotify ports.PeerNotifier) *DeleteContainerUseCase {
	return &DeleteContainerUseCase{containers: containers, peerNotify: peerNotify}
}

func (u *DeleteContainerUseCase) Execute(ctx context.Context, containerID string) (*container.Entity, error) {
	found, err := u.containers.GetContainerById(ctx, containerID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("container not found")
	}

	removed, err := u.containers.MarkContainerRemoved(ctx, containerID)
	if err != nil {
		return nil, err
	}
	if removed == nil {
		return nil, errors.New("container not found")
	}

	if u.peerNotify != nil {
		_ = u.peerNotify.RemoveWorkload(ctx, *removed)
	}

	return removed, nil
}
