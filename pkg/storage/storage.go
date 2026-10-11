// Package storage defines what the API server needs from its database.
// pkg/storage/postgres is the implementation.
package storage

import (
	"context"
	"errors"
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

// ErrNotFound is returned when a single object lookup finds nothing.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a conditional write lost (e.g. a single-use
// join token that was already consumed).
var ErrConflict = errors.New("conflict")

type CreateNodeParams struct {
	Name           string
	PublicIP       string
	PrivateIP      string
	AgentVersion   *string
	ContainerdSock string
}

type CreateContainerParams struct {
	NodeID string
	v1.CreateContainerRequest
}

// RuntimeUpdate is applied by agents' runtime reports. Nil identity fields
// keep their stored value.
type RuntimeUpdate struct {
	v1.ContainerRuntimeReport
	ScheduledAt *time.Time
	StartedAt   *time.Time
	StoppedAt   *time.Time
}

type ResourceUsage struct {
	RunningCount   int
	TotalCPU       float64
	TotalMemoryMiB int32
}

type ReclaimableSubnet struct {
	NodeID     string
	SubnetCIDR string
}

// Interface is the API server's persistence layer.
type Interface interface {
	// Tx runs fn in one database transaction.
	Tx(ctx context.Context, fn func(Interface) error) error

	GetNode(ctx context.Context, id string) (*v1.Node, error)
	GetNodeByName(ctx context.Context, name string) (*v1.Node, error)
	GetNodeByAPIKeyHash(ctx context.Context, hash string) (*v1.Node, error)
	ListNodes(ctx context.Context) ([]v1.Node, error)
	ListSchedulableNodes(ctx context.Context) ([]v1.Node, error)
	CreateNode(ctx context.Context, params CreateNodeParams) (*v1.Node, error)
	// HeartbeatNode only stamps last_heartbeat; it never changes status.
	HeartbeatNode(ctx context.Context, id string) (*v1.Node, error)
	SetNodeStatus(ctx context.Context, id string, status v1.NodeStatus) (*v1.Node, error)
	// MarkStaleNodesNotReady flips ready nodes whose last heartbeat (or
	// creation) is older than staleBefore and returns them.
	MarkStaleNodesNotReady(ctx context.Context, staleBefore time.Time) ([]v1.Node, error)
	EvictNode(ctx context.Context, id string) (*v1.Node, error)
	UpdateNodeHardware(ctx context.Context, id string, hw v1.NodeHardware) (*v1.Node, error)
	SetNodeAPIKey(ctx context.Context, id, hash string, expiresAt time.Time) error

	// LockOverlayAllocation serializes subnet allocation; call inside Tx.
	LockOverlayAllocation(ctx context.Context) error
	SetNodeOverlay(ctx context.Context, id, overlayIP, subnetCIDR string, status v1.NodeStatus) (*v1.Node, error)
	ClearNodeOverlay(ctx context.Context, id string) error
	ListActiveOverlaySubnets(ctx context.Context) ([]string, error)
	ListReclaimableOverlaySubnets(ctx context.Context) ([]ReclaimableSubnet, error)

	CreateJoinToken(ctx context.Context, hash string, singleUse bool, expiresAt *time.Time) (*v1.JoinToken, error)
	GetJoinTokenByHash(ctx context.Context, hash string) (*v1.JoinToken, error)
	ListJoinTokens(ctx context.Context) ([]v1.JoinToken, error)
	// MarkJoinTokenUsed returns ErrConflict for a consumed single-use token.
	MarkJoinTokenUsed(ctx context.Context, id, nodeID string) error

	CreateContainer(ctx context.Context, params CreateContainerParams) (*v1.Container, error)
	GetContainer(ctx context.Context, id string) (*v1.Container, error)
	ListContainers(ctx context.Context, opts v1.ContainerListOptions) ([]v1.Container, error)
	ScheduleContainer(ctx context.Context, id string) (*v1.Container, error)
	MarkContainerRemoved(ctx context.Context, id string) (*v1.Container, error)
	UpdateContainerRuntime(ctx context.Context, id string, update RuntimeUpdate) (*v1.Container, error)
	NodeResourceUsage(ctx context.Context, nodeID string) (*ResourceUsage, error)
	HasActiveContainerWithImage(ctx context.Context, nodeID, image string) (bool, error)
	// AddContainerEvent appends to the audit log; the store sets ID and
	// CreatedAt. ListContainerEvents returns newest first.
	AddContainerEvent(ctx context.Context, ev v1.ContainerEvent) error
	ListContainerEvents(ctx context.Context, containerID string) ([]v1.ContainerEvent, error)

	SeedOverlayPool(ctx context.Context, nodeID string, ips []string) error
	DeleteOverlayPool(ctx context.Context, nodeID string) error
	ListOverlayIPs(ctx context.Context, nodeID string, freeOnly bool) ([]v1.OverlayIP, error)
	CountFreeOverlayIPs(ctx context.Context, nodeID string) (int, error)
	// AllocateOverlayIP returns ErrNotFound when the node's pool is exhausted.
	AllocateOverlayIP(ctx context.Context, nodeID, containerID string) (string, error)
	ReleaseOverlayIP(ctx context.Context, ip string) error
}
