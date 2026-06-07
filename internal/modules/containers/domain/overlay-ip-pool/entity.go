package overlayippool

import (
	"net"
	"time"
)

type Entity struct {
	IP          net.IP     `json:"ip"`
	NodeID      string     `json:"node_id"`
	ContainerID *string    `json:"container_id,omitempty"`
	AllocatedAt *time.Time `json:"allocated_at,omitempty"`
}
