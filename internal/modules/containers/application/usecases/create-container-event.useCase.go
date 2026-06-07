package usecases

import (
	"context"
	"errors"

	containerevent "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container-event"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type CreateContainerEventUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewCreateContainerEventUseCase(containerRepository ports.ContainerRepositoryInterface) *CreateContainerEventUseCase {
	return &CreateContainerEventUseCase{containerRepository: containerRepository}
}

func (u *CreateContainerEventUseCase) Execute(ctx context.Context, input ports.CreateContainerEventInput) (*containerevent.Entity, error) {
	if input.ContainerID == "" {
		return nil, errors.New("container_id is required")
	}

	if input.NodeID == "" {
		return nil, errors.New("node_id is required")
	}

	createdEvent, err := u.containerRepository.CreateContainerEvent(ctx, input)
	if err != nil {
		return nil, err
	}

	if createdEvent == nil {
		return nil, errors.New("failed to create container event")
	}

	return createdEvent, nil
}
