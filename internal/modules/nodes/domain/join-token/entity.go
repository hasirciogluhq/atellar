package jointoken

import "time"

type JoinTokenEntity struct {
	ID        string     `json:"id"`
	SingleUse bool       `json:"single_use"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	UsedBy    *string    `json:"used_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type JoinTokenCreateResult struct {
	JoinTokenEntity
	Token string `json:"token"`
}
