package node

import (
	"net"
	"time"
)

type NodeStatus string

const (
	NodeStatusReady       NodeStatus = "ready"
	NodeStatusNotReady    NodeStatus = "not_ready"
	NodeStatusDown        NodeStatus = "down"
	NodeStatusMaintenance NodeStatus = "maintenance"
	NodeStatusEvicting    NodeStatus = "evicting"
	NodeStatusEvicted     NodeStatus = "evicted"
	NodeStatusPending     NodeStatus = "pending"
)

type NodeEntity struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	PublicIP      net.IP     `json:"public_ip" omitempty:"true"`
	PrivateIP     net.IP     `json:"private_ip" omitempty:"true"`
	Status        NodeStatus `json:"status" omitempty:"true" default:"pending"`
	LastHeartbeat *time.Time `json:"last_heartbeat" omitempty:"true"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
