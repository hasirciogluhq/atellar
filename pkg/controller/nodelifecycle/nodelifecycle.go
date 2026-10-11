// Package nodelifecycle marks nodes not_ready when their agent stops
// sending heartbeats. A later heartbeat flips them back (pkg/apiserver/nodes).
package nodelifecycle

import (
	"context"
	"log"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/events"
)

const (
	DefaultStaleAfter = 30 * time.Second
	DefaultInterval   = 5 * time.Second
)

// Store is the storage the controller needs.
type Store interface {
	MarkStaleNodesNotReady(ctx context.Context, staleBefore time.Time) ([]v1.Node, error)
}

type Controller struct {
	store      Store
	events     *events.Broker[v1.Node]
	staleAfter time.Duration
	interval   time.Duration
	now        func() time.Time
}

// New returns a controller. A zero staleAfter selects the default; a zero
// interval checks three times per staleAfter, at most every 5 seconds.
func New(store Store, nodeEvents *events.Broker[v1.Node], staleAfter, interval time.Duration) *Controller {
	if staleAfter <= 0 {
		staleAfter = DefaultStaleAfter
	}
	if interval <= 0 {
		interval = min(DefaultInterval, staleAfter/3)
	}
	return &Controller{store: store, events: nodeEvents, staleAfter: staleAfter, interval: interval, now: time.Now}
}

// Run checks every interval until ctx is done.
func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.Sync(ctx); err != nil && ctx.Err() == nil {
				log.Printf("nodelifecycle: %v", err)
			}
		}
	}
}

// Sync marks stale nodes once, publishes each change and returns the nodes
// it changed.
func (c *Controller) Sync(ctx context.Context) ([]v1.Node, error) {
	stale, err := c.store.MarkStaleNodesNotReady(ctx, c.now().Add(-c.staleAfter))
	if err != nil {
		return nil, err
	}
	for _, n := range stale {
		log.Printf("nodelifecycle: node %s (%s) missed heartbeats, marked not_ready", n.Name, n.ID)
		c.events.Publish(watch.Modified, n)
	}
	return stale, nil
}
