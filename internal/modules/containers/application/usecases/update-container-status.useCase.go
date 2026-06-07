package usecases

import (
	"context"
	"errors"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type UpdateContainerStatusUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewUpdateContainerStatusUseCase(containerRepository ports.ContainerRepositoryInterface) *UpdateContainerStatusUseCase {
	return &UpdateContainerStatusUseCase{containerRepository: containerRepository}
}

func (u *UpdateContainerStatusUseCase) Execute(ctx context.Context, containerID string, status container.Status) (*container.Entity, error) {
	updatedContainer, err := u.containerRepository.UpdateContainerStatus(ctx, containerID, status)
	if err != nil {
		return nil, err
	}

	if updatedContainer == nil {
		return nil, errors.New("container not found")
	}

	return updatedContainer, nil
}
