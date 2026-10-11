package apiserver_test

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
)

// TestContainerLifecycle walks one container through the reports an agent
// sends and checks what the API server records at each step.
func TestContainerLifecycle(t *testing.T) {
	f := apiservertest.New(t)
	agent := f.Context(t, apiservertest.NodeSelf)
	admin := f.Context(t, apiservertest.Admin)
	cs := f.Server.Containers()
	id := f.Container.ID

	freeBefore, _ := f.Store.CountFreeOverlayIPs(agent, f.NodeA.ID)

	withIP, err := cs.AllocateOverlayIP(agent, id)
	if err != nil {
		t.Fatal(err)
	}
	_, subnet, _ := net.ParseCIDR(f.NodeA.OverlaySubnet)
	if !subnet.Contains(net.ParseIP(withIP.OverlayIP)) {
		t.Fatalf("overlay ip %s outside node subnet %s", withIP.OverlayIP, f.NodeA.OverlaySubnet)
	}
	again, err := cs.AllocateOverlayIP(agent, id)
	if err != nil || again.OverlayIP != withIP.OverlayIP {
		t.Fatalf("allocation is not idempotent: %s then %s (%v)", withIP.OverlayIP, again.OverlayIP, err)
	}
	if free, _ := f.Store.CountFreeOverlayIPs(agent, f.NodeA.ID); free != freeBefore-1 {
		t.Fatalf("free ips %d, want %d", free, freeBefore-1)
	}

	restarts := int32(2)
	pid := int32(4242)
	running, err := cs.ReportRuntime(agent, id, v1.ContainerRuntimeReport{Status: v1.ContainerStatusRunning, TaskPID: &pid, RestartCount: &restarts})
	if err != nil {
		t.Fatal(err)
	}
	if running.StartedAt == nil || running.RestartCount != 2 || *running.TaskPID != 4242 {
		t.Fatalf("running report not recorded: %+v", running)
	}

	// A report without identity fields keeps the stored values.
	pulled, err := cs.ReportRuntime(agent, id, v1.ContainerRuntimeReport{Status: v1.ContainerStatusRunning})
	if err != nil {
		t.Fatal(err)
	}
	if pulled.OverlayIP != withIP.OverlayIP || pulled.RestartCount != 2 {
		t.Fatalf("report wiped fields: ip %q restarts %d", pulled.OverlayIP, pulled.RestartCount)
	}

	if _, err := cs.Delete(admin, id); err != nil {
		t.Fatal(err)
	}
	term, err := cs.ReportRuntime(agent, id, v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated})
	if err != nil {
		t.Fatal(err)
	}
	if term.StoppedAt == nil {
		t.Fatal("stopped_at not set")
	}
	if free, _ := f.Store.CountFreeOverlayIPs(agent, f.NodeA.ID); free != freeBefore {
		t.Fatalf("overlay ip not released: free %d, want %d", free, freeBefore)
	}

	active, err := cs.List(agent, v1.ContainerListOptions{NodeID: f.NodeA.ID, ActiveOnly: true})
	if err != nil || len(active) != 0 {
		t.Fatalf("terminated container still listed as active: %d (%v)", len(active), err)
	}
}

func TestReportRuntimeValidation(t *testing.T) {
	f := apiservertest.New(t)
	agent := f.Context(t, apiservertest.NodeSelf)
	cases := map[string]v1.ContainerRuntimeReport{
		"no status": {},
		"bad ip":    {Status: v1.ContainerStatusRunning, OverlayIP: "x"},
	}
	for name, report := range cases {
		_, err := f.Server.Containers().ReportRuntime(agent, f.Container.ID, report)
		if err := apiservertest.Check(err, api.ReasonBadRequest); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	_, err := f.Server.Containers().ReportRuntime(agent, "missing", v1.ContainerRuntimeReport{Status: v1.ContainerStatusRunning})
	if !api.IsNotFound(err) {
		t.Errorf("missing container: %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	neg := -1.0
	negMem := int32(-1)
	cases := map[string]v1.CreateContainerRequest{
		"no image":        {},
		"blank image":     {Image: "  "},
		"bad policy":      {Image: "x", RestartPolicy: "sometimes"},
		"negative cpu":    {Image: "x", CpuLimit: &neg},
		"negative memory": {Image: "x", MemoryLimitMiB: &negMem},
	}
	f := apiservertest.New(t)
	admin := f.Context(t, apiservertest.Admin)
	for name, req := range cases {
		_, err := f.Server.Containers().Create(admin, req)
		if err := apiservertest.Check(err, api.ReasonBadRequest); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestCreateSchedules checks a new container lands on a ready node, gets the
// default restart policy and is announced to watchers.
func TestCreateSchedules(t *testing.T) {
	f := apiservertest.New(t)
	admin := f.Context(t, apiservertest.Admin)
	created, err := f.Server.Containers().Create(admin, v1.CreateContainerRequest{Image: "docker.io/library/redis:7"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != v1.ContainerStatusScheduled || created.ScheduledAt == nil {
		t.Fatalf("status %s scheduled_at %v", created.Status, created.ScheduledAt)
	}
	if created.NodeID != f.NodeA.ID && created.NodeID != f.NodeB.ID {
		t.Fatalf("scheduled on unknown node %s", created.NodeID)
	}
	if created.RestartPolicy != v1.RestartPolicyOnFailure {
		t.Fatalf("restart policy %s", created.RestartPolicy)
	}
}

func TestJoinTokenCreate(t *testing.T) {
	f := apiservertest.New(t)
	admin := f.Context(t, apiservertest.Admin)
	jt := f.Server.JoinTokens()

	past := time.Now().Add(-time.Minute)
	if _, err := jt.Create(admin, v1.CreateJoinTokenRequest{ExpiresAt: &past}); api.ReasonOf(err) != api.ReasonBadRequest {
		t.Fatalf("past expiry: %v", err)
	}
	multi := false
	created, err := jt.Create(admin, v1.CreateJoinTokenRequest{SingleUse: &multi})
	if err != nil || created.SingleUse || created.Token == "" {
		t.Fatalf("multi use token: %+v %v", created, err)
	}
	list, err := jt.List(admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.Token != "" {
			t.Fatal("List exposes a plain token")
		}
	}
}

// TestReportAfterDelete covers which reports are accepted once a container
// is removed or terminated.
func TestReportAfterDelete(t *testing.T) {
	statuses := []v1.ContainerStatus{
		v1.ContainerStatusPulling, v1.ContainerStatusRunning, v1.ContainerStatusCrashed,
		v1.ContainerStatusStopped, v1.ContainerStatusTerminated,
	}
	for _, phase := range []v1.ContainerStatus{v1.ContainerStatusRemoved, v1.ContainerStatusTerminated} {
		for _, report := range statuses {
			t.Run(string(phase)+"/"+string(report), func(t *testing.T) {
				f := apiservertest.New(t)
				agent := f.Context(t, apiservertest.NodeSelf)
				if _, err := f.Server.Containers().Delete(f.Context(t, apiservertest.Admin), f.Container.ID); err != nil {
					t.Fatal(err)
				}
				if phase == v1.ContainerStatusTerminated {
					if _, err := f.Server.Containers().ReportRuntime(agent, f.Container.ID, v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated}); err != nil {
						t.Fatal(err)
					}
				}
				var want api.Reason
				if phase == v1.ContainerStatusTerminated || report != v1.ContainerStatusTerminated {
					want = api.ReasonConflict
				}
				_, err := f.Server.Containers().ReportRuntime(agent, f.Container.ID, v1.ContainerRuntimeReport{Status: report})
				if err := apiservertest.Check(err, want); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// TestContainerEvents checks the audit log written along the lifecycle.
func TestContainerEvents(t *testing.T) {
	f := apiservertest.New(t)
	admin := f.Context(t, apiservertest.Admin)
	agent := f.Context(t, apiservertest.NodeSelf)
	cs := f.Server.Containers()
	created, err := cs.Create(admin, v1.CreateContainerRequest{Image: "docker.io/library/redis:7"})
	if err != nil {
		t.Fatal(err)
	}
	if created.NodeID != f.NodeA.ID {
		// Events are reported by the owning node; use the fixture container
		// when the scheduler picked node B.
		created = f.Container
	}
	id := created.ID
	one, pid, digest := int32(1), int32(7), "sha256:abc"
	reports := []v1.ContainerRuntimeReport{
		{Status: v1.ContainerStatusPulling},
		{Status: v1.ContainerStatusPulling}, // repeated: no new event
		{Status: v1.ContainerStatusCreating, ImageDigest: &digest},
		{Status: v1.ContainerStatusRunning, TaskPID: &pid},
		{Status: v1.ContainerStatusCrashed, ExitCode: &one},
		{Status: v1.ContainerStatusPulling, RestartCount: &one},
	}
	for _, r := range reports {
		if _, err := cs.ReportRuntime(agent, id, r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cs.Delete(admin, id); err != nil {
		t.Fatal(err)
	}
	events, err := cs.ListEvents(admin, id)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		s := string(e.Event)
		if e.Message != nil {
			s += ":" + *e.Message
		}
		got = append(got, s)
	}
	want := []string{"pull_started", "pull_finished:sha256:abc", "started:pid 7", "crashed:exit code 1", "restart:restart 1", "pull_started", "removed"}
	if created.ID != f.Container.ID {
		want = append([]string{"scheduled"}, want...)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events\n got %v\nwant %v", got, want)
	}
}
