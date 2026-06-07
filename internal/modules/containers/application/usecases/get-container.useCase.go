package usecases

import (
	"context"
	"errors"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type GetContainerUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewGetContainerUseCase(containerRepository ports.ContainerRepositoryInterface) *GetContainerUseCase {
	return &GetContainerUseCase{containerRepository: containerRepository}
}

func (u *GetContainerUseCase) Execute(ctx context.Context, containerID string) (*container.Entity, error) {
	foundContainer, err := u.containerRepository.GetContainerById(ctx, containerID)
	if err != nil {
		return nil, err
	}

	if foundContainer == nil {
		return nil, errors.New("container not found")
	}

	return foundContainer, nil
}
