package usecases

import (
	"context"
	"errors"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type GetNodeWorkloadsUseCase struct {
	containers ports.ContainerRepositoryInterface
}

func NewGetNodeWorkloadsUseCase(containers ports.ContainerRepositoryInterface) *GetNodeWorkloadsUseCase {
	return &GetNodeWorkloadsUseCase{containers: containers}
}

func (u *GetNodeWorkloadsUseCase) Execute(ctx context.Context, nodeID string) ([]container.Entity, error) {
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}
	return u.containers.ListWorkloadsByNodeId(ctx, nodeID)
}
