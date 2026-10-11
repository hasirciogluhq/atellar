package workload

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/network/netns"
	"github.com/hasirciogluhq/atellar/pkg/runtime/containerd"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

type fakeRuntime struct {
	mu       sync.Mutex
	imageErr error
	runErr   error
	running  map[string]bool
	purged   []string
	runs     int
	onExit   func(id string, code int)
}

func (f *fakeRuntime) EnsureImage(context.Context, string, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return "sha256:abc", f.imageErr
}

func (f *fakeRuntime) RunContainer(_ context.Context, c *v1.Container, ip string) (*containerd.LocalContainer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.runErr != nil {
		return nil, f.runErr
	}
	f.runs++
	f.running[c.ID] = true
	return &containerd.LocalContainer{ContainerdID: c.ID, SnapshotKey: c.ID, TaskPID: 100, ImageDigest: "sha256:abc", OverlayIP: ip}, nil
}

func (f *fakeRuntime) PurgeState(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.running, id)
	f.purged = append(f.purged, id)
	return nil
}

func (f *fakeRuntime) TaskRunning(_ context.Context, id string) (bool, int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running[id], 100, nil
}

func (f *fakeRuntime) SubscribeExits(_ context.Context, h func(string, int)) error {
	f.onExit = h
	return nil
}

type fakeNetwork struct {
	mu       sync.Mutex
	setups   []netns.Config
	teardown []string
}

func (f *fakeNetwork) Setup(cfg netns.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setups = append(f.setups, cfg)
	return nil
}

func (f *fakeNetwork) Teardown(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.teardown = append(f.teardown, id)
}

type harness struct {
	f   *apiservertest.Fixture
	m   *Manager
	rt  *fakeRuntime
	net *fakeNetwork
	now time.Time
}

// newHarness runs the agent side against a real API server over HTTP,
// authenticated as node A.
func newHarness(t *testing.T, policy v1.RestartPolicy) *harness {
	t.Helper()
	f := apiservertest.New(t)
	srv, _ := transport.NewServer(transport.HTTP, f.Server, f.Auth)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx, ln) }()
	t.Cleanup(cancel)
	client, err := transport.NewClient(transport.HTTP, "http://"+ln.Addr().String(), func() string { return f.KeyA })
	if err != nil {
		t.Fatal(err)
	}

	// Replace the fixture's container with one using the requested policy.
	created, err := f.Store.CreateContainer(ctx, storage.CreateContainerParams{
		NodeID:                 f.NodeA.ID,
		CreateContainerRequest: v1.CreateContainerRequest{Image: "docker.io/library/nginx:1.27", RestartPolicy: policy},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Store.ScheduleContainer(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Store.UpdateContainerRuntime(ctx, f.Container.ID, storage.RuntimeUpdate{ContainerRuntimeReport: v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated}}); err != nil {
		t.Fatal(err)
	}
	f.Container = created

	h := &harness{f: f, rt: &fakeRuntime{running: map[string]bool{}}, net: &fakeNetwork{}, now: time.Now()}
	h.m = NewManager(Config{
		NodeID: f.NodeA.ID,
		Local:  func() (string, string) { return f.NodeA.OverlayIP, f.NodeA.OverlaySubnet },
	}, client.Containers(), h.rt, h.net)
	h.m.now = func() time.Time { return h.now }
	_ = h.m.rt.SubscribeExits(ctx, func(id string, code int) { h.m.onExit(ctx, id, code) })
	return h
}

// step runs one reconcile and waits for the work it started.
func (h *harness) step(t *testing.T) *v1.Container {
	t.Helper()
	h.m.Reconcile(context.Background())
	h.m.Wait()
	c, err := h.f.Store.GetContainer(context.Background(), h.f.Container.ID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStartReportsRunning(t *testing.T) {
	h := newHarness(t, v1.RestartPolicyOnFailure)
	c := h.step(t)
	if c.Status != v1.ContainerStatusRunning || c.OverlayIP == "" || c.StartedAt == nil || *c.TaskPID != 100 || *c.ImageDigest != "sha256:abc" {
		t.Fatalf("after start: %+v", c)
	}
	_, subnet, _ := net.ParseCIDR(h.f.NodeA.OverlaySubnet)
	if len(h.net.setups) != 1 || !subnet.Contains(net.ParseIP(h.net.setups[0].OverlayIP)) || h.net.setups[0].NodeOverlayIP != h.f.NodeA.OverlayIP {
		t.Fatalf("netns setup: %+v", h.net.setups)
	}

	// Steady state: the task is running, nothing else happens.
	c = h.step(t)
	if c.Status != v1.ContainerStatusRunning || h.rt.runs != 1 {
		t.Fatalf("steady state restarted the container: runs=%d status=%s", h.rt.runs, c.Status)
	}
}

func TestStartFailure(t *testing.T) {
	cases := map[v1.RestartPolicy]v1.ContainerStatus{
		v1.RestartPolicyNo:        v1.ContainerStatusFailed,
		v1.RestartPolicyOnFailure: v1.ContainerStatusBackoff,
		v1.RestartPolicyAlways:    v1.ContainerStatusBackoff,
	}
	for policy, want := range cases {
		t.Run(string(policy), func(t *testing.T) {
			h := newHarness(t, policy)
			h.rt.imageErr = errors.New("manifest unknown")
			c := h.step(t)
			if c.Status != want || c.ErrorMessage == nil || *c.ErrorMessage != "manifest unknown" || c.LastFailedAt == nil {
				t.Fatalf("after failure: status %s error %v", c.Status, c.ErrorMessage)
			}
			if len(h.net.teardown) == 0 || len(h.rt.purged) == 0 {
				t.Fatal("partial state not cleaned up")
			}

			// Inside the backoff window nothing happens; after it, the
			// agent retries with a higher restart count.
			h.rt.imageErr = nil
			if c = h.step(t); c.Status != want {
				t.Fatalf("retried inside backoff: %s", c.Status)
			}
			h.now = h.now.Add(time.Minute)
			c = h.step(t)
			if want == v1.ContainerStatusFailed {
				if c.Status != v1.ContainerStatusFailed {
					t.Fatalf("policy no retried: %s", c.Status)
				}
				return
			}
			if c.Status != v1.ContainerStatusRunning || c.RestartCount != 1 || c.ErrorMessage != nil {
				t.Fatalf("after retry: status %s restarts %d error %v", c.Status, c.RestartCount, c.ErrorMessage)
			}
		})
	}
}

// TestExit covers exit code x restart policy.
func TestExit(t *testing.T) {
	type want struct {
		status  v1.ContainerStatus
		restart bool
	}
	cases := []struct {
		code int
		want map[v1.RestartPolicy]want
	}{
		{0, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo:        {v1.ContainerStatusStopped, false},
			v1.RestartPolicyOnFailure: {v1.ContainerStatusStopped, false},
			v1.RestartPolicyAlways:    {v1.ContainerStatusStopped, true},
		}},
		{137, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo:        {v1.ContainerStatusCrashed, false},
			v1.RestartPolicyOnFailure: {v1.ContainerStatusCrashed, true},
			v1.RestartPolicyAlways:    {v1.ContainerStatusCrashed, true},
		}},
	}
	for _, tc := range cases {
		for policy, w := range tc.want {
			t.Run(string(policy)+"/"+string(w.status), func(t *testing.T) {
				h := newHarness(t, policy)
				h.step(t)
				h.rt.mu.Lock()
				delete(h.rt.running, h.f.Container.ID)
				h.rt.mu.Unlock()
				h.rt.onExit(h.f.Container.ID, tc.code)

				c, _ := h.f.Store.GetContainer(context.Background(), h.f.Container.ID)
				if c.Status != w.status || c.ExitCode == nil || int(*c.ExitCode) != tc.code {
					t.Fatalf("after exit: status %s exit %v", c.Status, c.ExitCode)
				}
				h.now = h.now.Add(time.Hour)
				c = h.step(t)
				restarted := c.Status == v1.ContainerStatusRunning
				if restarted != w.restart {
					t.Fatalf("restarted = %v (status %s), want %v", restarted, c.Status, w.restart)
				}
				if restarted && c.RestartCount != 1 {
					t.Fatalf("restart count %d", c.RestartCount)
				}
			})
		}
	}
}

func TestDeleteCleansUp(t *testing.T) {
	h := newHarness(t, v1.RestartPolicyAlways)
	running := h.step(t)
	free, _ := h.f.Store.CountFreeOverlayIPs(context.Background(), h.f.NodeA.ID)

	admin := h.f.Context(t, apiservertest.Admin)
	if _, err := h.f.Server.Containers().Delete(admin, h.f.Container.ID); err != nil {
		t.Fatal(err)
	}
	c := h.step(t)
	if c.Status != v1.ContainerStatusTerminated {
		t.Fatalf("status %s", c.Status)
	}
	if after, _ := h.f.Store.CountFreeOverlayIPs(context.Background(), h.f.NodeA.ID); after != free+1 {
		t.Fatalf("overlay ip %s not released: %d -> %d", running.OverlayIP, free, after)
	}
	// The exit caused by the cleanup must not be reported as a crash.
	h.rt.onExit(h.f.Container.ID, 137)
	if c, _ := h.f.Store.GetContainer(context.Background(), h.f.Container.ID); c.Status != v1.ContainerStatusTerminated {
		t.Fatalf("exit after cleanup changed status to %s", c.Status)
	}
}

func TestAgentRestart(t *testing.T) {
	for _, alive := range []bool{true, false} {
		t.Run(map[bool]string{true: "task alive", false: "task gone"}[alive], func(t *testing.T) {
			h := newHarness(t, v1.RestartPolicyOnFailure)
			h.step(t)
			// A new manager has no local state, like a restarted agent.
			fresh := NewManager(h.m.cfg, h.m.api, h.rt, h.net)
			fresh.now = h.m.now
			h.m = fresh
			if !alive {
				h.rt.mu.Lock()
				delete(h.rt.running, h.f.Container.ID)
				h.rt.mu.Unlock()
			}
			c := h.step(t)
			if alive && (c.Status != v1.ContainerStatusRunning || h.rt.runs != 1) {
				t.Fatalf("adopt: status %s runs %d", c.Status, h.rt.runs)
			}
			if !alive && c.Status != v1.ContainerStatusCrashed {
				t.Fatalf("lost task: status %s", c.Status)
			}
		})
	}
}
