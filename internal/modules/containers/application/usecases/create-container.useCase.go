package usecases

import (
	"context"
	"errors"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type CreateContainerUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewCreateContainerUseCase(containerRepository ports.ContainerRepositoryInterface) *CreateContainerUseCase {
	return &CreateContainerUseCase{containerRepository: containerRepository}
}

func (u *CreateContainerUseCase) Execute(ctx context.Context, input ports.CreateContainerInput) (*container.Entity, error) {
	if input.NodeID == "" {
		return nil, errors.New("node_id is required")
	}

	if input.Image == "" {
		return nil, errors.New("image is required")
	}

	createdContainer, err := u.containerRepository.CreateContainer(ctx, input)
	if err != nil {
		return nil, err
	}

	if createdContainer == nil {
		return nil, errors.New("failed to create container")
	}

	return createdContainer, nil
}
