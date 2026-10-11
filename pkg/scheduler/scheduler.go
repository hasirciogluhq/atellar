// Package scheduler picks a node for a new container.
package scheduler

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

// ErrNoNodes means no node can take the container right now.
var ErrNoNodes = errors.New("no schedulable nodes available")

// Store is the read-only slice of storage the scheduler needs.
type Store interface {
	ListSchedulableNodes(ctx context.Context) ([]v1.Node, error)
	NodeResourceUsage(ctx context.Context, nodeID string) (*storage.ResourceUsage, error)
	CountFreeOverlayIPs(ctx context.Context, nodeID string) (int, error)
	HasActiveContainerWithImage(ctx context.Context, nodeID, image string) (bool, error)
}

type Scheduler struct {
	store Store
}

func New(store Store) *Scheduler {
	return &Scheduler{store: store}
}

type Request struct {
	Image          string
	CpuLimit       *float64
	MemoryLimitMiB *int32
}

// SelectNode filters ready nodes by free CPU, memory and overlay IPs, keeps
// one active copy of an image per node, and picks the least loaded node.
func (s *Scheduler) SelectNode(ctx context.Context, req Request) (string, error) {
	candidates, err := s.store.ListSchedulableNodes(ctx)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", ErrNoNodes
	}

	var requestCPU float64
	if req.CpuLimit != nil {
		requestCPU = *req.CpuLimit
	}
	var requestMem int32
	if req.MemoryLimitMiB != nil {
		requestMem = *req.MemoryLimitMiB
	}

	bestID, bestCount := "", 0
	for _, n := range candidates {
		if n.CpuCores == nil || n.MemoryTotalMiB == nil {
			continue
		}

		usage, err := s.store.NodeResourceUsage(ctx, n.ID)
		if err != nil {
			continue
		}
		if usage.TotalCPU+requestCPU > float64(*n.CpuCores) {
			continue
		}
		if usage.TotalMemoryMiB+requestMem > *n.MemoryTotalMiB {
			continue
		}

		freeIPs, err := s.store.CountFreeOverlayIPs(ctx, n.ID)
		if err != nil || freeIPs == 0 {
			continue
		}

		hasImage, err := s.store.HasActiveContainerWithImage(ctx, n.ID, req.Image)
		if err != nil || hasImage {
			continue
		}

		if bestID == "" || usage.RunningCount < bestCount {
			bestID, bestCount = n.ID, usage.RunningCount
		}
	}

	if bestID == "" {
		return "", fmt.Errorf("%w: no node satisfies constraints for image %s", ErrNoNodes, req.Image)
	}
	return bestID, nil
}
