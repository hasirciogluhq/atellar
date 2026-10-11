// Package workload runs the containers the API server scheduled on this
// node and reports every lifecycle step back.
package workload

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/network/netns"
	"github.com/hasirciogluhq/atellar/pkg/runtime/containerd"
)

// Runtime is the container runtime (pkg/runtime/containerd in production).
type Runtime interface {
	EnsureImage(ctx context.Context, ref, digest string) (string, error)
	RunContainer(ctx context.Context, c *v1.Container, overlayIP string) (*containerd.LocalContainer, error)
	PurgeState(ctx context.Context, id string) error
	TaskRunning(ctx context.Context, id string) (bool, int32, error)
	SubscribeExits(ctx context.Context, handler func(id string, exitCode int)) error
}

// Network wires a container's network namespace (pkg/network/netns).
type Network interface {
	Setup(netns.Config) error
	Teardown(id string)
}

// Netns is the production Network.
type Netns struct{}

func (Netns) Setup(cfg netns.Config) error { return netns.Setup(cfg) }
func (Netns) Teardown(id string)           { netns.Teardown(id) }

type Config struct {
	NodeID     string
	BridgeName string
	MTU        int
	// Local returns the node's overlay IP and subnet.
	Local func() (overlayIP, subnet string)
	// PollInterval is the fallback resync period (default 15s).
	PollInterval time.Duration
}

// Manager converges local containers to the API server's view. Each
// container is handled by at most one goroutine at a time.
type Manager struct {
	cfg     Config
	api     api.ContainerInterface
	rt      Runtime
	net     Network
	now     func() time.Time
	trigger chan struct{}

	mu       sync.Mutex
	local    map[string]containerd.LocalContainer
	inflight map[string]bool
	wake     *time.Timer
	wakeAt   time.Time
	work     sync.WaitGroup
}

func NewManager(cfg Config, containers api.ContainerInterface, rt Runtime, network Network) *Manager {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 15 * time.Second
	}
	return &Manager{
		cfg: cfg, api: containers, rt: rt, net: network, now: time.Now,
		trigger:  make(chan struct{}, 1),
		local:    map[string]containerd.LocalContainer{},
		inflight: map[string]bool{},
	}
}

// Trigger requests a reconcile soon. It never blocks.
func (m *Manager) Trigger() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

// Run reconciles on every trigger and every PollInterval until ctx is done.
func (m *Manager) Run(ctx context.Context) {
	if err := m.rt.SubscribeExits(ctx, func(id string, code int) { m.onExit(ctx, id, code) }); err != nil {
		log.Printf("workload: subscribe to task exits: %v", err)
	}
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	for {
		m.Reconcile(ctx)
		select {
		case <-ctx.Done():
			m.work.Wait()
			return
		case <-ticker.C:
		case <-m.trigger:
		}
	}
}

// Reconcile lists this node's containers once and dispatches the work each
// needs. It returns without waiting for that work.
func (m *Manager) Reconcile(ctx context.Context) {
	items, err := m.api.List(ctx, v1.ContainerListOptions{NodeID: m.cfg.NodeID, ActiveOnly: true})
	if err != nil {
		log.Printf("workload: list containers: %v", err)
		return
	}
	now := m.now()
	listed := make(map[string]bool, len(items))
	for _, c := range items {
		listed[c.ID] = true
		act, delay := decide(c, now)
		if act == actionNone {
			continue
		}
		if delay > 0 {
			m.wakeIn(delay)
			continue
		}
		m.dispatch(ctx, c, act)
	}

	// Containers the server no longer lists are gone: clean up quietly.
	m.mu.Lock()
	var orphans []string
	for id := range m.local {
		if !listed[id] && !m.inflight[id] {
			orphans = append(orphans, id)
		}
	}
	m.mu.Unlock()
	for _, id := range orphans {
		m.dispatch(ctx, v1.Container{ID: id}, actionCleanup)
	}
}

// Wait blocks until all dispatched work has finished. Used by tests.
func (m *Manager) Wait() { m.work.Wait() }

func (m *Manager) dispatch(ctx context.Context, c v1.Container, act action) {
	m.mu.Lock()
	if m.inflight[c.ID] {
		m.mu.Unlock()
		return
	}
	m.inflight[c.ID] = true
	m.mu.Unlock()

	m.work.Add(1)
	go func() {
		defer m.work.Done()
		defer func() {
			m.mu.Lock()
			delete(m.inflight, c.ID)
			m.mu.Unlock()
		}()
		switch act {
		case actionStart:
			m.start(ctx, c, c.RestartCount)
		case actionRestart:
			m.start(ctx, c, c.RestartCount+1)
		case actionVerify:
			m.verify(ctx, c)
		case actionCleanup:
			m.cleanup(ctx, c.ID, c.Status == v1.ContainerStatusRemoved)
		}
	}()
}

// wakeIn schedules a reconcile after d unless one is due sooner.
func (m *Manager) wakeIn(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	at := m.now().Add(d)
	if m.wake != nil && !m.wakeAt.IsZero() && m.wakeAt.Before(at) && m.wakeAt.After(m.now()) {
		return
	}
	if m.wake != nil {
		m.wake.Stop()
	}
	m.wakeAt = at
	m.wake = time.AfterFunc(d, m.Trigger)
}

func (m *Manager) report(ctx context.Context, id string, r v1.ContainerRuntimeReport) error {
	_, err := m.api.ReportRuntime(ctx, id, r)
	return err
}

// start runs the pipeline: pull, allocate an address, wire the network
// namespace, create and start the task. Every step is reported.
func (m *Manager) start(ctx context.Context, c v1.Container, restarts int32) {
	m.forget(c.ID)

	if err := m.report(ctx, c.ID, v1.ContainerRuntimeReport{Status: v1.ContainerStatusPulling, RestartCount: &restarts}); err != nil {
		m.abort(c.ID, "report pulling", err)
		return
	}
	digest, err := m.rt.EnsureImage(ctx, c.Image, deref(c.ImageDigest))
	if err != nil {
		m.fail(ctx, c, restarts, err)
		return
	}
	if err := m.report(ctx, c.ID, v1.ContainerRuntimeReport{Status: v1.ContainerStatusCreating, ImageDigest: &digest}); err != nil {
		m.abort(c.ID, "report creating", err)
		return
	}
	withIP, err := m.api.AllocateOverlayIP(ctx, c.ID)
	if err != nil {
		m.fail(ctx, c, restarts, err)
		return
	}
	overlayIP, subnet := "", ""
	if m.cfg.Local != nil {
		overlayIP, subnet = m.cfg.Local()
	}
	if err := m.net.Setup(netns.Config{
		ContainerID:       c.ID,
		OverlayIP:         withIP.OverlayIP,
		NodeOverlayIP:     overlayIP,
		NodeOverlaySubnet: subnet,
		BridgeName:        m.cfg.BridgeName,
		MTU:               m.cfg.MTU,
	}); err != nil {
		m.fail(ctx, c, restarts, err)
		return
	}
	run, err := m.rt.RunContainer(ctx, withIP, withIP.OverlayIP)
	if err != nil {
		m.fail(ctx, c, restarts, err)
		return
	}

	m.mu.Lock()
	m.local[c.ID] = *run
	m.mu.Unlock()
	if err := m.report(ctx, c.ID, v1.ContainerRuntimeReport{
		Status:       v1.ContainerStatusRunning,
		ContainerdID: &run.ContainerdID,
		SnapshotKey:  &run.SnapshotKey,
		TaskPID:      &run.TaskPID,
		ImageDigest:  &run.ImageDigest,
		OverlayIP:    run.OverlayIP,
	}); err != nil {
		m.abort(c.ID, "report running", err)
	}
}

// fail undoes a partial start and reports backoff or failed.
func (m *Manager) fail(ctx context.Context, c v1.Container, restarts int32, cause error) {
	log.Printf("workload: start %s: %v", c.ID, cause)
	m.net.Teardown(c.ID)
	_ = m.rt.PurgeState(ctx, c.ID)
	now := m.now()
	msg := cause.Error()
	if err := m.report(ctx, c.ID, v1.ContainerRuntimeReport{
		Status:       afterFailure(c.RestartPolicy),
		ErrorMessage: &msg,
		RestartCount: &restarts,
		LastFailedAt: &now,
	}); err != nil {
		m.abort(c.ID, "report failure", err)
		return
	}
	m.wakeIn(retryDelay(restarts))
}

// abort handles a report the server refused. A conflict means the container
// was deleted meanwhile; the next reconcile cleans it up.
func (m *Manager) abort(id, step string, err error) {
	if api.ReasonOf(err) == api.ReasonConflict {
		m.Trigger()
		return
	}
	log.Printf("workload: %s %s: %v", step, id, err)
}

// verify adopts a running task after an agent restart, or reports the
// container crashed when its task is gone.
func (m *Manager) verify(ctx context.Context, c v1.Container) {
	running, pid, err := m.rt.TaskRunning(ctx, c.ID)
	if err != nil {
		log.Printf("workload: inspect %s: %v", c.ID, err)
		return
	}
	if running {
		m.mu.Lock()
		if _, known := m.local[c.ID]; !known {
			m.local[c.ID] = containerd.LocalContainer{
				ContainerdID: c.ID, SnapshotKey: c.ID, TaskPID: pid,
				ImageDigest: deref(c.ImageDigest), OverlayIP: c.OverlayIP,
			}
		}
		m.mu.Unlock()
		return
	}
	m.forget(c.ID)
	now := m.now()
	msg := "task not found on node"
	if err := m.report(ctx, c.ID, v1.ContainerRuntimeReport{Status: v1.ContainerStatusCrashed, ErrorMessage: &msg, LastFailedAt: &now}); err != nil {
		m.abort(c.ID, "report crashed", err)
		return
	}
	m.Trigger()
}

// cleanup stops and deletes everything local. report is false for orphans
// the server already forgot.
func (m *Manager) cleanup(ctx context.Context, id string, report bool) {
	m.forget(id)
	m.net.Teardown(id)
	if err := m.rt.PurgeState(ctx, id); err != nil {
		log.Printf("workload: purge %s: %v", id, err)
		return
	}
	if !report {
		return
	}
	if err := m.report(ctx, id, v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated}); err != nil &&
		api.ReasonOf(err) != api.ReasonConflict {
		log.Printf("workload: report terminated %s: %v", id, err)
	}
}

// forget drops the local record so the exit of a task the agent stops on
// purpose is not reported as a crash.
func (m *Manager) forget(id string) {
	m.mu.Lock()
	delete(m.local, id)
	m.mu.Unlock()
}

// onExit reports a task that ended on its own.
func (m *Manager) onExit(ctx context.Context, id string, code int) {
	m.mu.Lock()
	_, ok := m.local[id]
	delete(m.local, id)
	m.mu.Unlock()
	if !ok {
		return
	}
	exit := int32(code)
	r := v1.ContainerRuntimeReport{Status: v1.ContainerStatusStopped, ExitCode: &exit}
	if code != 0 {
		now := m.now()
		r.Status, r.LastFailedAt = v1.ContainerStatusCrashed, &now
	}
	if err := m.report(ctx, id, r); err != nil && !errors.Is(err, context.Canceled) {
		m.abort(id, "report exit", err)
		return
	}
	m.Trigger()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
