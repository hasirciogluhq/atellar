package usecases

import (
	"context"
	"errors"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type UpdateNodeHardwareUseCase struct {
	nodes ports.NodeRepositoryInterface
}

func NewUpdateNodeHardwareUseCase(nodes ports.NodeRepositoryInterface) *UpdateNodeHardwareUseCase {
	return &UpdateNodeHardwareUseCase{nodes: nodes}
}

func (u *UpdateNodeHardwareUseCase) Execute(ctx context.Context, nodeID string, input ports.UpdateNodeHardwareInput) (*node.NodeEntity, error) {
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}
	if input.CpuCores <= 0 || input.MemoryTotalMiB <= 0 {
		return nil, errors.New("cpu_cores and memory_total_mib are required")
	}

	updated, err := u.nodes.UpdateNodeHardware(ctx, nodeID, input)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("node not found")
	}
	return updated, nil
}
