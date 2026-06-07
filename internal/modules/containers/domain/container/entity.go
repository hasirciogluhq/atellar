package container

import (
	"net"
	"time"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusScheduled  Status = "scheduled"
	StatusPulling    Status = "pulling"
	StatusCreating   Status = "creating"
	StatusRunning    Status = "running"
	StatusStopped    Status = "stopped"
	StatusCrashed    Status = "crashed"
	StatusBackoff    Status = "backoff"
	StatusFailed     Status = "failed"
	StatusRemoved    Status = "removed"
	StatusTerminated Status = "terminated"
)

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

type Entity struct {
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
	OverlayIP      net.IP            `json:"overlay_ip,omitempty"`
	ExposedPorts   []ExposedPort     `json:"exposed_ports,omitempty"`
	CpuLimit       *float64          `json:"cpu_limit,omitempty"`
	CpuShares      int32             `json:"cpu_shares"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib,omitempty"`
	Status         Status            `json:"status"`
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
