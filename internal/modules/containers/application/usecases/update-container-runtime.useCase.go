package usecases

import (
	"context"
	"errors"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type UpdateContainerRuntimeUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewUpdateContainerRuntimeUseCase(containerRepository ports.ContainerRepositoryInterface) *UpdateContainerRuntimeUseCase {
	return &UpdateContainerRuntimeUseCase{containerRepository: containerRepository}
}

func (u *UpdateContainerRuntimeUseCase) Execute(ctx context.Context, containerID string, input ports.UpdateContainerRuntimeInput) (*container.Entity, error) {
	updatedContainer, err := u.containerRepository.UpdateContainerRuntime(ctx, containerID, input)
	if err != nil {
		return nil, err
	}

	if updatedContainer == nil {
		return nil, errors.New("container not found")
	}

	return updatedContainer, nil
}
