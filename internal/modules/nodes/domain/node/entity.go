package node

import (
	"net"
	"time"
)

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

type NodeEntity struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	PublicIP           net.IP     `json:"public_ip,omitempty"`
	PrivateIP          net.IP     `json:"private_ip,omitempty"`
	OverlayIP          net.IP     `json:"overlay_ip,omitempty"`
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
