package services

import (
	"context"
	"errors"
	"fmt"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
	nodeports "github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
)

type Scheduler struct {
	nodes      nodeports.NodeRepositoryInterface
	containers ports.ContainerRepositoryInterface
}

func NewScheduler(nodes nodeports.NodeRepositoryInterface, containers ports.ContainerRepositoryInterface) *Scheduler {
	return &Scheduler{nodes: nodes, containers: containers}
}

type ScheduleRequest struct {
	Image          string
	CpuLimit       *float64
	MemoryLimitMiB *int32
}

func (s *Scheduler) SelectNode(ctx context.Context, req ScheduleRequest) (string, error) {
	candidates, err := s.nodes.ListSchedulableNodes(ctx)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("no schedulable nodes available")
	}

	requestCPU := float64(0)
	if req.CpuLimit != nil {
		requestCPU = *req.CpuLimit
	}
	requestMem := int32(0)
	if req.MemoryLimitMiB != nil {
		requestMem = *req.MemoryLimitMiB
	}

	type scored struct {
		nodeID string
		count  int
	}
	var best *scored

	for _, n := range candidates {
		if n.CpuCores == nil || n.MemoryTotalMiB == nil {
			continue
		}

		usage, err := s.containers.NodeResourceUsage(ctx, n.ID)
		if err != nil {
			continue
		}

		if usage.TotalCPU+requestCPU > float64(*n.CpuCores) {
			continue
		}
		if usage.TotalMemoryMiB+requestMem > *n.MemoryTotalMiB {
			continue
		}

		freeIPs, err := s.containers.CountFreeOverlayIPsByNodeId(ctx, n.ID)
		if err != nil || freeIPs == 0 {
			continue
		}

		hasImage, err := s.containers.HasContainerWithImageOnNode(ctx, n.ID, req.Image)
		if err != nil || hasImage {
			continue
		}

		candidate := &scored{nodeID: n.ID, count: usage.RunningCount}
		if best == nil || candidate.count < best.count {
			best = candidate
		}
	}

	if best == nil {
		return "", fmt.Errorf("no node satisfies scheduler constraints for image %s", req.Image)
	}

	return best.nodeID, nil
}

func DefaultRestartPolicy(p container.RestartPolicy) container.RestartPolicy {
	if p == "" {
		return container.RestartPolicyOnFailure
	}
	return p
}
