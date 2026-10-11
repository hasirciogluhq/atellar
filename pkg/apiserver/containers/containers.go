// Package containers implements api.ContainerInterface: deploy, delete and
// the runtime reports agents send while running a container.
package containers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/registry"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/auth/authz"
	"github.com/hasirciogluhq/atellar/pkg/scheduler"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

type Registry struct {
	deps      *registry.Deps
	scheduler *scheduler.Scheduler
	now       func() time.Time
}

var _ api.ContainerInterface = (*Registry)(nil)

func New(deps *registry.Deps, sched *scheduler.Scheduler) *Registry {
	return &Registry{deps: deps, scheduler: sched, now: time.Now}
}

func (r *Registry) List(ctx context.Context, opts v1.ContainerListOptions) ([]v1.Container, error) {
	if err := r.authorizeRead(ctx, opts); err != nil {
		return nil, err
	}
	items, err := r.deps.Store.ListContainers(ctx, opts)
	return items, registry.StorageError(err, "containers", opts.NodeID)
}

func (r *Registry) Get(ctx context.Context, id string) (*v1.Container, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterReadContainers); err != nil {
		return nil, err
	}
	c, err := r.deps.Store.GetContainer(ctx, id)
	return c, registry.StorageError(err, "container", id)
}

// Watch filters live events by node only: a container that turns
// terminated is still delivered so watchers can drop it.
func (r *Registry) Watch(ctx context.Context, opts v1.ContainerListOptions) (watch.Interface[v1.Container], error) {
	if err := r.authorizeRead(ctx, opts); err != nil {
		return nil, err
	}
	var filter func(v1.Container) bool
	if opts.NodeID != "" {
		filter = func(c v1.Container) bool { return c.NodeID == opts.NodeID }
	}
	return registry.Watch(ctx, r.deps.ContainerEvents, filter, func(ctx context.Context) ([]v1.Container, error) {
		items, err := r.deps.Store.ListContainers(ctx, opts)
		return items, registry.StorageError(err, "containers", opts.NodeID)
	})
}

// Create schedules the container on a node and publishes it; the node's
// agent picks it up from its watch.
func (r *Registry) Create(ctx context.Context, req v1.CreateContainerRequest) (*v1.Container, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterDeployContainer); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, registry.BadRequest(err)
	}

	nodeID, err := r.scheduler.SelectNode(ctx, scheduler.Request{
		Image:          req.Image,
		CpuLimit:       req.CpuLimit,
		MemoryLimitMiB: req.MemoryLimitMiB,
	})
	if errors.Is(err, scheduler.ErrNoNodes) {
		return nil, api.Unavailable("%v", err)
	}
	if err != nil {
		return nil, registry.StorageError(err, "scheduler", req.Image)
	}

	var scheduled *v1.Container
	err = r.deps.Store.Tx(ctx, func(tx storage.Interface) error {
		created, err := tx.CreateContainer(ctx, storage.CreateContainerParams{NodeID: nodeID, CreateContainerRequest: req})
		if err != nil {
			return registry.StorageError(err, "container", req.Image)
		}
		scheduled, err = tx.ScheduleContainer(ctx, created.ID)
		return registry.StorageError(err, "container", created.ID)
	})
	if err != nil {
		return nil, err
	}

	r.record(ctx, scheduled, v1.ContainerEventScheduled, nil)
	r.deps.ContainerEvents.Publish(watch.Added, *scheduled)
	return scheduled, nil
}

// Delete marks the container removed; the agent stops it and reports
// terminated.
func (r *Registry) Delete(ctx context.Context, id string) (*v1.Container, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterDeleteContainer); err != nil {
		return nil, err
	}
	removed, err := r.deps.Store.MarkContainerRemoved(ctx, id)
	if err != nil {
		return nil, registry.StorageError(err, "container", id)
	}
	r.record(ctx, removed, v1.ContainerEventRemoved, nil)
	r.deps.ContainerEvents.Publish(watch.Modified, *removed)
	return removed, nil
}

func (r *Registry) ListEvents(ctx context.Context, id string) ([]v1.ContainerEvent, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterReadContainerEvent); err != nil {
		return nil, err
	}
	events, err := r.deps.Store.ListContainerEvents(ctx, id)
	return events, registry.StorageError(err, "container", id)
}

// ReportRuntime records a lifecycle step from the owning agent. A
// terminated container gives its overlay IP back to the node pool.
func (r *Registry) ReportRuntime(ctx context.Context, id string, report v1.ContainerRuntimeReport) (*v1.Container, error) {
	current, err := r.owned(ctx, authz.ActionAgentReportRuntime, id)
	if err != nil {
		return nil, err
	}
	if err := report.Validate(); err != nil {
		return nil, registry.BadRequest(err)
	}
	// A removed container may only finish; a terminated one is final. This
	// stops a pipeline that raced with Delete from bringing it back.
	switch {
	case current.Status == v1.ContainerStatusTerminated:
		return nil, api.Conflict("container %s is terminated", id)
	case current.Status == v1.ContainerStatusRemoved && report.Status != v1.ContainerStatusTerminated:
		return nil, api.Conflict("container %s is being removed; only terminated may be reported", id)
	}

	update := storage.RuntimeUpdate{ContainerRuntimeReport: report}
	now := r.now()
	switch report.Status {
	case v1.ContainerStatusRunning:
		update.StartedAt = &now
	case v1.ContainerStatusStopped, v1.ContainerStatusCrashed, v1.ContainerStatusFailed, v1.ContainerStatusTerminated:
		update.StoppedAt = &now
	}

	updated, err := r.deps.Store.UpdateContainerRuntime(ctx, id, update)
	if err != nil {
		return nil, registry.StorageError(err, "container", id)
	}

	if updated.Status == v1.ContainerStatusTerminated && current.Status != v1.ContainerStatusTerminated && updated.OverlayIP != "" {
		if err := r.deps.Store.ReleaseOverlayIP(ctx, updated.OverlayIP); err != nil {
			log.Printf("apiserver: release overlay ip %s of %s: %v", updated.OverlayIP, id, err)
		}
	}

	for _, ev := range lifecycleEvents(current, report) {
		r.record(ctx, updated, ev.typ, ev.msg)
	}
	r.deps.ContainerEvents.Publish(watch.Modified, *updated)
	return updated, nil
}

type lifecycleEvent struct {
	typ v1.ContainerEventType
	msg *string
}

// lifecycleEvents turns a runtime report into audit events. Only status
// changes are recorded, so repeated reports do not flood the log.
func lifecycleEvents(prev *v1.Container, report v1.ContainerRuntimeReport) []lifecycleEvent {
	var out []lifecycleEvent
	if report.RestartCount != nil && *report.RestartCount > prev.RestartCount {
		out = append(out, lifecycleEvent{v1.ContainerEventRestart, msgf("restart %d", *report.RestartCount)})
	}
	if report.Status == prev.Status {
		return out
	}
	exit := func() *string {
		switch {
		case report.ErrorMessage != nil:
			return report.ErrorMessage
		case report.ExitCode != nil:
			return msgf("exit code %d", *report.ExitCode)
		}
		return nil
	}
	switch report.Status {
	case v1.ContainerStatusPulling:
		out = append(out, lifecycleEvent{v1.ContainerEventPullStarted, nil})
	case v1.ContainerStatusCreating:
		out = append(out, lifecycleEvent{v1.ContainerEventPullFinished, report.ImageDigest})
	case v1.ContainerStatusRunning:
		var msg *string
		if report.TaskPID != nil {
			msg = msgf("pid %d", *report.TaskPID)
		}
		out = append(out, lifecycleEvent{v1.ContainerEventStarted, msg})
	case v1.ContainerStatusStopped:
		out = append(out, lifecycleEvent{v1.ContainerEventStopped, exit()})
	case v1.ContainerStatusCrashed:
		out = append(out, lifecycleEvent{v1.ContainerEventCrashed, exit()})
	case v1.ContainerStatusBackoff:
		out = append(out, lifecycleEvent{v1.ContainerEventBackoff, exit()})
	case v1.ContainerStatusFailed:
		out = append(out, lifecycleEvent{v1.ContainerEventFailed, exit()})
	case v1.ContainerStatusTerminated:
		out = append(out, lifecycleEvent{v1.ContainerEventTerminated, nil})
	}
	return out
}

func msgf(format string, args ...any) *string {
	s := fmt.Sprintf(format, args...)
	return &s
}

// record writes an audit event. A failure is logged, never returned: the
// state change it describes already happened.
func (r *Registry) record(ctx context.Context, c *v1.Container, typ v1.ContainerEventType, msg *string) {
	err := r.deps.Store.AddContainerEvent(ctx, v1.ContainerEvent{ContainerID: c.ID, NodeID: c.NodeID, Event: typ, Message: msg})
	if err != nil {
		log.Printf("apiserver: record %s event for %s: %v", typ, c.ID, err)
	}
}

// AllocateOverlayIP reserves an address from the node's pool. It is
// idempotent: a container that already has an IP keeps it.
func (r *Registry) AllocateOverlayIP(ctx context.Context, id string) (*v1.Container, error) {
	current, err := r.owned(ctx, authz.ActionAgentAllocateOverlayIP, id)
	if err != nil {
		return nil, err
	}
	if current.OverlayIP != "" {
		return current, nil
	}

	var updated *v1.Container
	err = r.deps.Store.Tx(ctx, func(tx storage.Interface) error {
		ip, err := tx.AllocateOverlayIP(ctx, current.NodeID, id)
		if errors.Is(err, storage.ErrNotFound) {
			return api.Unavailable("no free overlay ip on node %s", current.NodeID)
		}
		if err != nil {
			return registry.StorageError(err, "overlay pool", current.NodeID)
		}
		updated, err = tx.UpdateContainerRuntime(ctx, id, storage.RuntimeUpdate{
			ContainerRuntimeReport: v1.ContainerRuntimeReport{OverlayIP: ip, Status: current.Status, RestartCount: &current.RestartCount},
		})
		return registry.StorageError(err, "container", id)
	})
	if err != nil {
		return nil, err
	}

	r.deps.ContainerEvents.Publish(watch.Modified, *updated)
	return updated, nil
}

// authorizeRead lets operators read every container and agents read only
// the containers scheduled on their own node.
func (r *Registry) authorizeRead(ctx context.Context, opts v1.ContainerListOptions) error {
	self := authn.NodeFrom(ctx)
	if self == nil {
		return r.deps.Authorize(ctx, authz.ActionClusterReadContainers)
	}
	if err := r.deps.Authorize(ctx, authz.ActionAgentReadWorkloads); err != nil {
		return err
	}
	if opts.NodeID != self.ID {
		return api.Forbidden("node %s may only read its own containers", self.ID)
	}
	return nil
}

// owned authorizes an agent call and checks the container is on its node.
func (r *Registry) owned(ctx context.Context, action authz.Action, id string) (*v1.Container, error) {
	if err := r.deps.Authorize(ctx, action); err != nil {
		return nil, err
	}
	self := authn.NodeFrom(ctx)
	if self == nil {
		return nil, api.Forbidden("only node agents may call this")
	}
	current, err := r.deps.Store.GetContainer(ctx, id)
	if err != nil {
		return nil, registry.StorageError(err, "container", id)
	}
	if current.NodeID != self.ID {
		return nil, api.Forbidden("container %s does not belong to node %s", id, self.ID)
	}
	return current, nil
}
