package usecases

import (
	"context"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type ListContainersUseCase struct {
	containerRepository ports.ContainerRepositoryInterface
}

func NewListContainersUseCase(containerRepository ports.ContainerRepositoryInterface) *ListContainersUseCase {
	return &ListContainersUseCase{containerRepository: containerRepository}
}

func (u *ListContainersUseCase) Execute(ctx context.Context, nodeID string) ([]container.Entity, error) {
	if nodeID != "" {
		return u.containerRepository.ListContainersByNodeId(ctx, nodeID)
	}

	return u.containerRepository.ListContainers(ctx)
}
