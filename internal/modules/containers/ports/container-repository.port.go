package ports

import (
	"context"
	"net"
	"time"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	containerevent "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container-event"
	overlayippool "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/overlay-ip-pool"
)

type CreateContainerInput struct {
	NodeID         string
	Image          string
	Command        []string
	Entrypoint     []string
	Env            map[string]string
	WorkingDir     *string
	CpuLimit       *float64
	CpuShares      *int32
	MemoryLimitMiB *int32
	RestartPolicy  container.RestartPolicy
	ContainerdNs   string
}

type UpdateContainerRuntimeInput struct {
	ContainerdID *string
	SnapshotKey  *string
	TaskPID      *int32
	ImageDigest  *string
	OverlayIP    net.IP
	Status       container.Status
	ExitCode     *int32
	ErrorMessage *string
	RestartCount *int32
	ScheduledAt  *time.Time
	StartedAt    *time.Time
	StoppedAt    *time.Time
}

type CreateContainerEventInput struct {
	ContainerID string
	NodeID      string
	Event       containerevent.EventType
	Message     *string
	Metadata    map[string]any
}

type ContainerRepositoryInterface interface {
	CreateContainer(ctx context.Context, input CreateContainerInput) (*container.Entity, error)
	GetContainerById(ctx context.Context, containerID string) (*container.Entity, error)
	ListContainers(ctx context.Context) ([]container.Entity, error)
	ListContainersByNodeId(ctx context.Context, nodeID string) ([]container.Entity, error)
	UpdateContainerStatus(ctx context.Context, containerID string, status container.Status) (*container.Entity, error)
	UpdateContainerRuntime(ctx context.Context, containerID string, input UpdateContainerRuntimeInput) (*container.Entity, error)
	CreateContainerEvent(ctx context.Context, input CreateContainerEventInput) (*containerevent.Entity, error)
	ListContainerEventsByContainerId(ctx context.Context, containerID string) ([]containerevent.Entity, error)
	CreateOverlayIPPoolEntry(ctx context.Context, ip net.IP, nodeID string) (*overlayippool.Entity, error)
	ListFreeOverlayIPsByNodeId(ctx context.Context, nodeID string) ([]overlayippool.Entity, error)
	ListOverlayIPsByNodeId(ctx context.Context, nodeID string) ([]overlayippool.Entity, error)
	AllocateOverlayIP(ctx context.Context, ip net.IP, containerID string) (*overlayippool.Entity, error)
	ReleaseOverlayIP(ctx context.Context, ip net.IP) error
}
