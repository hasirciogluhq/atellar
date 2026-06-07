package containerevent

import "time"

type EventType string

const (
	EventScheduled    EventType = "scheduled"
	EventPullStarted  EventType = "pull_started"
	EventPullFinished EventType = "pull_finished"
	EventPullFailed   EventType = "pull_failed"
	EventCreated      EventType = "created"
	EventStarted      EventType = "started"
	EventStopped      EventType = "stopped"
	EventCrashed      EventType = "crashed"
	EventTerminated   EventType = "terminated"
	EventRestart      EventType = "restart"
)

type Entity struct {
	ID          string         `json:"id"`
	ContainerID string         `json:"container_id"`
	NodeID      string         `json:"node_id"`
	Event       EventType      `json:"event"`
	Message     *string        `json:"message,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}
