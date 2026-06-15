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
	Status         container.Status
}

type DeployContainerInput struct {
	Image          string
	Command        []string
	Entrypoint     []string
	Env            map[string]string
	WorkingDir     *string
	CpuLimit       *float64
	CpuShares      *int32
	MemoryLimitMiB *int32
	RestartPolicy  container.RestartPolicy
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
	LastFailedAt *time.Time
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

type NodeResourceUsage struct {
	RunningCount   int
	TotalCPU       float64
	TotalMemoryMiB int32
}

type ContainerRepositoryInterface interface {
	CreateContainer(ctx context.Context, input CreateContainerInput) (*container.Entity, error)
	GetContainerById(ctx context.Context, containerID string) (*container.Entity, error)
	ListContainers(ctx context.Context) ([]container.Entity, error)
	ListContainersByNodeId(ctx context.Context, nodeID string) ([]container.Entity, error)
	ListWorkloadsByNodeId(ctx context.Context, nodeID string) ([]container.Entity, error)
	ScheduleContainer(ctx context.Context, containerID string) (*container.Entity, error)
	MarkContainerRemoved(ctx context.Context, containerID string) (*container.Entity, error)
	NodeResourceUsage(ctx context.Context, nodeID string) (*NodeResourceUsage, error)
	HasContainerWithImageOnNode(ctx context.Context, nodeID, image string) (bool, error)
	UpdateContainerStatus(ctx context.Context, containerID string, status container.Status) (*container.Entity, error)
	UpdateContainerRuntime(ctx context.Context, containerID string, input UpdateContainerRuntimeInput) (*container.Entity, error)
	CreateContainerEvent(ctx context.Context, input CreateContainerEventInput) (*containerevent.Entity, error)
	ListContainerEventsByContainerId(ctx context.Context, containerID string) ([]containerevent.Entity, error)
	CreateOverlayIPPoolEntry(ctx context.Context, ip net.IP, nodeID string) (*overlayippool.Entity, error)
	DeleteOverlayIPPoolByNodeId(ctx context.Context, nodeID string) error
	ListFreeOverlayIPsByNodeId(ctx context.Context, nodeID string) ([]overlayippool.Entity, error)
	ListOverlayIPsByNodeId(ctx context.Context, nodeID string) ([]overlayippool.Entity, error)
	CountFreeOverlayIPsByNodeId(ctx context.Context, nodeID string) (int, error)
	AllocateOverlayIP(ctx context.Context, ip net.IP, containerID string) (*overlayippool.Entity, error)
	AllocateFirstFreeOverlayIP(ctx context.Context, nodeID, containerID string) (*overlayippool.Entity, error)
	ReleaseOverlayIP(ctx context.Context, ip net.IP) error
}
