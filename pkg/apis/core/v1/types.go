// Package v1 holds the Atellar core API objects. These types are the wire
// format for every transport and the shape stored by pkg/storage.
package v1

import "time"

type NodeStatus string

const (
	NodeStatusPending     NodeStatus = "pending"
	NodeStatusReady       NodeStatus = "ready"
	NodeStatusNotReady    NodeStatus = "not_ready"
	NodeStatusMaintenance NodeStatus = "maintenance"
	NodeStatusEvicting    NodeStatus = "evicting"
	NodeStatusEvicted     NodeStatus = "evicted"
	NodeStatusDown        NodeStatus = "down"
)

// Node is a machine running the Atellar agent.
type Node struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	PublicIP           string     `json:"public_ip,omitempty"`
	PrivateIP          string     `json:"private_ip,omitempty"`
	OverlayIP          string     `json:"overlay_ip,omitempty"`
	OverlaySubnet      string     `json:"overlay_subnet,omitempty"`
	Status             NodeStatus `json:"status"`
	LastHeartbeat      *time.Time `json:"last_heartbeat,omitempty"`
	AgentVersion       *string    `json:"agent_version,omitempty"`
	ContainerdSock     string     `json:"containerd_sock"`
	CpuCores           *int32     `json:"cpu_cores,omitempty"`
	MemoryTotalMiB     *int32     `json:"memory_total_mib,omitempty"`
	DiskTotalGiB       *int32     `json:"disk_total_gib,omitempty"`
	Hostname           *string    `json:"hostname,omitempty"`
	OS                 *string    `json:"os,omitempty"`
	Arch               *string    `json:"arch,omitempty"`
	KernelVersion      *string    `json:"kernel_version,omitempty"`
	HardwareReportedAt *time.Time `json:"hardware_reported_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// NodeHardware is reported by the agent on start and periodically.
type NodeHardware struct {
	CpuCores       int32  `json:"cpu_cores"`
	MemoryTotalMiB int32  `json:"memory_total_mib"`
	DiskTotalGiB   int32  `json:"disk_total_gib"`
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	KernelVersion  string `json:"kernel_version"`
}

// RegisterNodeRequest joins a machine to the cluster with a join token.
type RegisterNodeRequest struct {
	JoinToken      string `json:"join_token"`
	Name           string `json:"name,omitempty"`
	PublicIP       string `json:"public_ip,omitempty"`
	PrivateIP      string `json:"private_ip,omitempty"`
	AgentVersion   string `json:"agent_version,omitempty"`
	ContainerdSock string `json:"containerd_sock,omitempty"`
}

type RegisterNodeResponse struct {
	Node            Node      `json:"node"`
	NodeAPIKey      string    `json:"node_api_key"`
	APIKeyExpiresAt time.Time `json:"api_key_expires_at"`
}

type NodeAPIKey struct {
	NodeAPIKey      string    `json:"node_api_key"`
	APIKeyExpiresAt time.Time `json:"api_key_expires_at"`
}

type JoinToken struct {
	ID        string     `json:"id"`
	SingleUse bool       `json:"single_use"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	UsedBy    *string    `json:"used_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	// Token is the plain secret. Only set once, in the create response.
	Token string `json:"token,omitempty"`
}

type CreateJoinTokenRequest struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	SingleUse *bool      `json:"single_use,omitempty"`
}

type ContainerStatus string

const (
	ContainerStatusPending    ContainerStatus = "pending"    // written to DB, node has not seen it yet
	ContainerStatusScheduled  ContainerStatus = "scheduled"  // assigned to node, not started yet
	ContainerStatusPulling    ContainerStatus = "pulling"    // pulling image
	ContainerStatusCreating   ContainerStatus = "creating"   // containerd container created, task not started
	ContainerStatusRunning    ContainerStatus = "running"    // task started, PID available
	ContainerStatusStopped    ContainerStatus = "stopped"    // graceful stop
	ContainerStatusCrashed    ContainerStatus = "crashed"    // exit_code != 0
	ContainerStatusBackoff    ContainerStatus = "backoff"    // waiting before retry
	ContainerStatusFailed     ContainerStatus = "failed"     // start failed and the restart policy is "no"
	ContainerStatusRemoved    ContainerStatus = "removed"    // delete requested, agent cleanup pending
	ContainerStatusTerminated ContainerStatus = "terminated" // deleted and cleaned up
)

// Active reports whether the container still occupies node resources or
// needs agent attention.
func (s ContainerStatus) Active() bool {
	switch s {
	case ContainerStatusPending, ContainerStatusScheduled, ContainerStatusPulling,
		ContainerStatusCreating, ContainerStatusRunning, ContainerStatusBackoff:
		return true
	}
	return false
}

type RestartPolicy string

const (
	RestartPolicyNo        RestartPolicy = "no"
	RestartPolicyAlways    RestartPolicy = "always"
	RestartPolicyOnFailure RestartPolicy = "on-failure"
)

type ExposedPort struct {
	Proto    string `json:"proto"`
	Port     int    `json:"port"`
	HostPort *int   `json:"host_port,omitempty"`
}

// Container is a single workload scheduled on one node.
type Container struct {
	ID             string            `json:"id"`
	NodeID         string            `json:"node_id"`
	ContainerdNs   string            `json:"containerd_ns"`
	ContainerdID   *string           `json:"containerd_id,omitempty"`
	SnapshotKey    *string           `json:"snapshot_key,omitempty"`
	TaskPID        *int32            `json:"task_pid,omitempty"`
	Image          string            `json:"image"`
	ImageDigest    *string           `json:"image_digest,omitempty"`
	Command        []string          `json:"command,omitempty"`
	Entrypoint     []string          `json:"entrypoint,omitempty"`
	Env            map[string]string `json:"env"`
	WorkingDir     *string           `json:"working_dir,omitempty"`
	OverlayIP      string            `json:"overlay_ip,omitempty"`
	ExposedPorts   []ExposedPort     `json:"exposed_ports,omitempty"`
	CpuLimit       *float64          `json:"cpu_limit,omitempty"`
	CpuShares      int32             `json:"cpu_shares"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib,omitempty"`
	Status         ContainerStatus   `json:"status"`
	ExitCode       *int32            `json:"exit_code,omitempty"`
	ErrorMessage   *string           `json:"error_message,omitempty"`
	RestartCount   int32             `json:"restart_count"`
	RestartPolicy  RestartPolicy     `json:"restart_policy"`
	LastFailedAt   *time.Time        `json:"last_failed_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	StoppedAt      *time.Time        `json:"stopped_at,omitempty"`
}

// CreateContainerRequest deploys a container; the scheduler picks the node.
type CreateContainerRequest struct {
	Image          string            `json:"image"`
	Command        []string          `json:"command,omitempty"`
	Entrypoint     []string          `json:"entrypoint,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	WorkingDir     *string           `json:"working_dir,omitempty"`
	CpuLimit       *float64          `json:"cpu_limit,omitempty"`
	CpuShares      *int32            `json:"cpu_shares,omitempty"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib,omitempty"`
	RestartPolicy  RestartPolicy     `json:"restart_policy,omitempty"`
}

// ContainerRuntimeReport is sent by the owning agent at every lifecycle step.
type ContainerRuntimeReport struct {
	ContainerdID *string         `json:"containerd_id,omitempty"`
	SnapshotKey  *string         `json:"snapshot_key,omitempty"`
	TaskPID      *int32          `json:"task_pid,omitempty"`
	ImageDigest  *string         `json:"image_digest,omitempty"`
	OverlayIP    string          `json:"overlay_ip,omitempty"`
	Status       ContainerStatus `json:"status"`
	ExitCode     *int32          `json:"exit_code,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	RestartCount *int32          `json:"restart_count,omitempty"`
	LastFailedAt *time.Time      `json:"last_failed_at,omitempty"`
}

// ContainerListOptions filters container lists and watches.
type ContainerListOptions struct {
	NodeID string `json:"node_id,omitempty"`
	// ActiveOnly limits the result to containers the node agent still has to
	// act on (everything except terminated).
	ActiveOnly bool `json:"active_only,omitempty"`
}

type ContainerEventType string

const (
	ContainerEventScheduled    ContainerEventType = "scheduled"
	ContainerEventPullStarted  ContainerEventType = "pull_started"
	ContainerEventPullFinished ContainerEventType = "pull_finished"
	ContainerEventPullFailed   ContainerEventType = "pull_failed"
	ContainerEventCreated      ContainerEventType = "created"
	ContainerEventStarted      ContainerEventType = "started"
	ContainerEventStopped      ContainerEventType = "stopped"
	ContainerEventCrashed      ContainerEventType = "crashed"
	ContainerEventTerminated   ContainerEventType = "terminated"
	ContainerEventRestart      ContainerEventType = "restart"
	ContainerEventBackoff      ContainerEventType = "backoff"
	ContainerEventFailed       ContainerEventType = "failed"
	ContainerEventRemoved      ContainerEventType = "removed"
)

// ContainerEvent is an audit record of a container lifecycle step.
type ContainerEvent struct {
	ID          string             `json:"id"`
	ContainerID string             `json:"container_id"`
	NodeID      string             `json:"node_id"`
	Event       ContainerEventType `json:"event"`
	Message     *string            `json:"message,omitempty"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
}

// OverlayIP is one entry of a node's overlay address pool.
type OverlayIP struct {
	IP          string     `json:"ip"`
	NodeID      string     `json:"node_id"`
	ContainerID *string    `json:"container_id,omitempty"`
	AllocatedAt *time.Time `json:"allocated_at,omitempty"`
}
