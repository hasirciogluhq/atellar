package usecases

import (
	"context"
	"errors"

	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/application/services"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type DeployContainerUseCase struct {
	scheduler  *services.Scheduler
	containers ports.ContainerRepositoryInterface
	peerNotify ports.PeerNotifier
}

func NewDeployContainerUseCase(
	scheduler *services.Scheduler,
	containers ports.ContainerRepositoryInterface,
	peerNotify ports.PeerNotifier,
) *DeployContainerUseCase {
	return &DeployContainerUseCase{
		scheduler:  scheduler,
		containers: containers,
		peerNotify: peerNotify,
	}
}

func (u *DeployContainerUseCase) Execute(ctx context.Context, input ports.DeployContainerInput) (*container.Entity, error) {
	if input.Image == "" {
		return nil, errors.New("image is required")
	}

	nodeID, err := u.scheduler.SelectNode(ctx, services.ScheduleRequest{
		Image:          input.Image,
		CpuLimit:       input.CpuLimit,
		MemoryLimitMiB: input.MemoryLimitMiB,
	})
	if err != nil {
		return nil, err
	}

	created, err := u.containers.CreateContainer(ctx, ports.CreateContainerInput{
		NodeID:         nodeID,
		Image:          input.Image,
		Command:        input.Command,
		Entrypoint:     input.Entrypoint,
		Env:            input.Env,
		WorkingDir:     input.WorkingDir,
		CpuLimit:       input.CpuLimit,
		CpuShares:      input.CpuShares,
		MemoryLimitMiB: input.MemoryLimitMiB,
		RestartPolicy:  services.DefaultRestartPolicy(input.RestartPolicy),
		ContainerdNs:   "atellar",
		Status:         container.StatusPending,
	})
	if err != nil {
		return nil, err
	}

	scheduled, err := u.containers.ScheduleContainer(ctx, created.ID)
	if err != nil {
		return nil, err
	}
	if scheduled == nil {
		return nil, errors.New("failed to schedule container")
	}

	if u.peerNotify != nil {
		_ = u.peerNotify.NotifyContainerEvent(ctx, agentregistry.PeerEventContainerScheduled, *scheduled)
		_ = u.peerNotify.DispatchWorkload(ctx, *scheduled)
	}

	return scheduled, nil
}
