package agentregistry

import (
	"sync"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
)

const reconcileTriggerMethod = "reconcile.trigger"

type agentConnection struct {
	mu   sync.Mutex
	send func(*atellarv1.ServerEnvelope) error
}

func (c *agentConnection) Send(envelope *atellarv1.ServerEnvelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.send(envelope)
}

type Registry struct {
	mu          sync.RWMutex
	connections map[string]*agentConnection
}

func NewRegistry() *Registry {
	return &Registry{
		connections: make(map[string]*agentConnection),
	}
}

func (r *Registry) Register(nodeID string, send func(*atellarv1.ServerEnvelope) error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connections[nodeID] = &agentConnection{send: send}
}

func (r *Registry) Unregister(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.connections, nodeID)
}

func (r *Registry) ConnectedNodeIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.connections))
	for nodeID := range r.connections {
		ids = append(ids, nodeID)
	}

	return ids
}
