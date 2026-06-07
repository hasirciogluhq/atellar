package usecases

import (
	"context"
	"errors"
	"time"

	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type ReportContainerRuntimeUseCase struct {
	containers ports.ContainerRepositoryInterface
	peerNotify ports.PeerNotifier
}

func NewReportContainerRuntimeUseCase(containers ports.ContainerRepositoryInterface, peerNotify ports.PeerNotifier) *ReportContainerRuntimeUseCase {
	return &ReportContainerRuntimeUseCase{containers: containers, peerNotify: peerNotify}
}

type ReportContainerRuntimeInput struct {
	NodeID       string
	ContainerID  string
	Runtime      ports.UpdateContainerRuntimeInput
}

func (u *ReportContainerRuntimeUseCase) Execute(ctx context.Context, input ReportContainerRuntimeInput) (*container.Entity, error) {
	found, err := u.containers.GetContainerById(ctx, input.ContainerID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("container not found")
	}
	if found.NodeID != input.NodeID {
		return nil, errors.New("container does not belong to this node")
	}

	runtime := input.Runtime
	now := time.Now()
	switch runtime.Status {
	case container.StatusRunning:
		if runtime.StartedAt == nil {
			runtime.StartedAt = &now
		}
	case container.StatusStopped, container.StatusCrashed, container.StatusFailed, container.StatusTerminated:
		if runtime.StoppedAt == nil {
			runtime.StoppedAt = &now
		}
	}

	updated, err := u.containers.UpdateContainerRuntime(ctx, input.ContainerID, runtime)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("container not found")
	}

	if u.peerNotify != nil {
		if updated.OverlayIP != nil {
			_ = u.peerNotify.NotifyContainerEvent(ctx, agentregistry.PeerEventContainerUpdated, *updated)
		}
		if event := peerEventForStatus(updated.Status); event != "" {
			_ = u.peerNotify.NotifyContainerEvent(ctx, event, *updated)
		}
	}

	return updated, nil
}

func peerEventForStatus(status container.Status) string {
	switch status {
	case container.StatusScheduled, container.StatusPending:
		return agentregistry.PeerEventContainerScheduled
	case container.StatusRunning:
		return agentregistry.PeerEventContainerStarted
	case container.StatusStopped:
		return agentregistry.PeerEventContainerStopped
	case container.StatusTerminated:
		return agentregistry.PeerEventContainerTerminated
	default:
		return ""
	}
}
