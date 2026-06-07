package node

import "time"

type NodeAPIKeyResult struct {
	NodeAPIKey      string    `json:"node_api_key"`
	APIKeyExpiresAt time.Time `json:"api_key_expires_at"`
}

type RegisterNodeResult struct {
	Node            NodeEntity `json:"node"`
	NodeAPIKey      string     `json:"node_api_key"`
	APIKeyExpiresAt time.Time  `json:"api_key_expires_at"`
}
