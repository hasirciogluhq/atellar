package client

import "time"

type RegisterNodeRequest struct {
	Name           string `json:"name,omitempty"`
	PublicIP       string `json:"public_ip,omitempty"`
	PrivateIP      string `json:"private_ip,omitempty"`
	AgentVersion   string `json:"agent_version,omitempty"`
	ContainerdSock string `json:"containerd_sock,omitempty"`
}

type Node struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	PublicIP           string     `json:"public_ip,omitempty"`
	PrivateIP          string     `json:"private_ip,omitempty"`
	OverlayIP          string     `json:"overlay_ip,omitempty"`
	OverlaySubnet      string     `json:"overlay_subnet,omitempty"`
	Status             string     `json:"status"`
	LastHeartbeat      *time.Time `json:"last_heartbeat,omitempty"`
	AgentVersion       *string    `json:"agent_version,omitempty"`
	ContainerdSock     string     `json:"containerd_sock,omitempty"`
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

type RegisterNodeResult struct {
	Node            Node      `json:"node"`
	NodeAPIKey      string    `json:"node_api_key"`
	APIKeyExpiresAt time.Time `json:"api_key_expires_at"`
}

type NodeAPIKeyResult struct {
	NodeAPIKey      string    `json:"node_api_key"`
	APIKeyExpiresAt time.Time `json:"api_key_expires_at"`
}

type CreateJoinTokenRequest struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	SingleUse *bool      `json:"single_use,omitempty"`
}

type JoinToken struct {
	ID        string     `json:"id"`
	SingleUse bool       `json:"single_use"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	UsedBy    *string    `json:"used_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Token     string     `json:"token,omitempty"`
}

type UpdateNodeOverlayRequest struct {
	OverlayIP     string `json:"overlay_ip"`
	OverlaySubnet string `json:"overlay_subnet"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type ExposedPort struct {
	Proto    string `json:"proto"`
	Port     int    `json:"port"`
	HostPort *int   `json:"host_port,omitempty"`
}

type Container struct {
	ID             string            `json:"id"`
	NodeID         string            `json:"node_id"`
	ContainerdNs   string            `json:"containerd_ns,omitempty"`
	ContainerdID   *string           `json:"containerd_id,omitempty"`
	SnapshotKey    *string           `json:"snapshot_key,omitempty"`
	TaskPID        *int32            `json:"task_pid,omitempty"`
	Image          string            `json:"image"`
	ImageDigest    *string           `json:"image_digest,omitempty"`
	Command        []string          `json:"command,omitempty"`
	Entrypoint     []string          `json:"entrypoint,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	WorkingDir     *string           `json:"working_dir,omitempty"`
	OverlayIP      string            `json:"overlay_ip,omitempty"`
	ExposedPorts   []ExposedPort     `json:"exposed_ports,omitempty"`
	CpuLimit       *float64          `json:"cpu_limit,omitempty"`
	CpuShares      int32             `json:"cpu_shares,omitempty"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib,omitempty"`
	Status         string            `json:"status"`
	ExitCode       *int32            `json:"exit_code,omitempty"`
	ErrorMessage   *string           `json:"error_message,omitempty"`
	RestartCount   int32             `json:"restart_count,omitempty"`
	RestartPolicy  string            `json:"restart_policy,omitempty"`
	LastFailedAt   *time.Time        `json:"last_failed_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	ScheduledAt    *time.Time        `json:"scheduled_at,omitempty"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	StoppedAt      *time.Time        `json:"stopped_at,omitempty"`
}

type CreateContainerRequest struct {
	Image          string            `json:"image"`
	Command        []string          `json:"command,omitempty"`
	Entrypoint     []string          `json:"entrypoint,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	WorkingDir     *string           `json:"working_dir,omitempty"`
	CpuLimit       *float64          `json:"cpu_limit,omitempty"`
	CpuShares      *int32            `json:"cpu_shares,omitempty"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib,omitempty"`
	RestartPolicy  string            `json:"restart_policy,omitempty"`
}

type UpdateContainerStatusRequest struct {
	Status string `json:"status"`
}

type UpdateContainerRuntimeRequest struct {
	ContainerdID *string `json:"containerd_id,omitempty"`
	SnapshotKey  *string `json:"snapshot_key,omitempty"`
	TaskPID      *int32  `json:"task_pid,omitempty"`
	ImageDigest  *string `json:"image_digest,omitempty"`
	OverlayIP    string  `json:"overlay_ip,omitempty"`
	Status       string  `json:"status"`
	ExitCode     *int32  `json:"exit_code,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
	RestartCount *int32  `json:"restart_count,omitempty"`
}

type ContainerEvent struct {
	ID          string         `json:"id"`
	ContainerID string         `json:"container_id"`
	NodeID      string         `json:"node_id"`
	Event       string         `json:"event"`
	Message     *string        `json:"message,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type CreateContainerEventRequest struct {
	NodeID   string         `json:"node_id"`
	Event    string         `json:"event"`
	Message  *string        `json:"message,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type OverlayIPPoolEntry struct {
	IP          string     `json:"ip"`
	NodeID      string     `json:"node_id"`
	ContainerID *string    `json:"container_id,omitempty"`
	AllocatedAt *time.Time `json:"allocated_at,omitempty"`
}

type CreateOverlayIPRequest struct {
	IP     string `json:"ip"`
	NodeID string `json:"node_id"`
}

type AllocateOverlayIPRequest struct {
	ContainerID string `json:"container_id"`
}
