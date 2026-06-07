package usecases

import (
	"context"

	containerevent "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container-event"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type ListContainerEventsUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewListContainerEventsUseCase(containerRepository ports.ContainerRepositoryInterface) *ListContainerEventsUseCase {
	return &ListContainerEventsUseCase{containerRepository: containerRepository}
}

func (u *ListContainerEventsUseCase) Execute(ctx context.Context, containerID string) ([]containerevent.Entity, error) {
	return u.containerRepository.ListContainerEventsByContainerId(ctx, containerID)
}
