package jointoken

import "time"

type JoinTokenEntity struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
