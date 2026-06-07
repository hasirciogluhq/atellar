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
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	PublicIP      string    `json:"public_ip,omitempty"`
	PrivateIP     string    `json:"private_ip,omitempty"`
	OverlayIP     string    `json:"overlay_ip,omitempty"`
	OverlaySubnet string    `json:"overlay_subnet,omitempty"`
	Status        string    `json:"status"`
	ContainerdSock string   `json:"containerd_sock,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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

type Container struct {
	ID        string `json:"id"`
	NodeID    string `json:"node_id"`
	Image     string `json:"image"`
	Status    string `json:"status"`
	OverlayIP string `json:"overlay_ip,omitempty"`
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
