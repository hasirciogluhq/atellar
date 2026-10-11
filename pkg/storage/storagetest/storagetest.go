// Package storagetest is the conformance suite every storage.Interface
// implementation must pass. The memory store runs it on every test run; the
// PostgreSQL store runs it when ATELLAR_TEST_DATABASE_URL is set.
package storagetest

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

// Open returns an empty store for one test.
type Open func(t *testing.T) storage.Interface

// Run executes the suite. Subtests run sequentially so a database backed
// store can reset between them.
func Run(t *testing.T, open Open) {
	tests := []struct {
		name string
		fn   func(*testing.T, storage.Interface)
	}{
		{"Nodes", testNodes},
		{"NodeStatus", testNodeStatus},
		{"StaleNodes", testStaleNodes},
		{"APIKeys", testAPIKeys},
		{"Overlay", testOverlay},
		{"OverlayPool", testOverlayPool},
		{"JoinTokens", testJoinTokens},
		{"Containers", testContainers},
		{"RuntimeUpdates", testRuntimeUpdates},
		{"ResourceUsage", testResourceUsage},
		{"Events", testEvents},
		{"Tx", testTx},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					f, ok := r.(failure)
					if !ok {
						panic(r)
					}
					t.Fatalf("unexpected error: %v", f.err)
				}
			}()
			tc.fn(t, open(t))
		})
	}
}

var ctx = context.Background()

// must panics on error; Run turns the panic into a test failure. This keeps
// call sites to one line: must(s.GetNode(ctx, id)).
func must[T any](v T, err error) T {
	if err != nil {
		panic(failure{err})
	}
	return v
}

type failure struct{ err error }

func ptr[T any](v T) *T { return &v }

func node(t *testing.T, s storage.Interface, name string) *v1.Node {
	t.Helper()
	return must(s.CreateNode(ctx, storage.CreateNodeParams{Name: name, PrivateIP: "192.168.0.1"}))
}

func readyNode(t *testing.T, s storage.Interface, name, subnet, ip string) *v1.Node {
	t.Helper()
	n := node(t, s, name)
	must(s.SetNodeOverlay(ctx, n.ID, ip, subnet, v1.NodeStatusReady))
	return must(s.UpdateNodeHardware(ctx, n.ID, v1.NodeHardware{CpuCores: 4, MemoryTotalMiB: 4096, DiskTotalGiB: 50}))
}

func container(t *testing.T, s storage.Interface, nodeID, image string, req v1.CreateContainerRequest) *v1.Container {
	t.Helper()
	req.Image = image
	return must(s.CreateContainer(ctx, storage.CreateContainerParams{NodeID: nodeID, CreateContainerRequest: req}))
}

func testNodes(t *testing.T, s storage.Interface) {
	n := must(s.CreateNode(ctx, storage.CreateNodeParams{Name: "a", PublicIP: "203.0.113.1", PrivateIP: "10.1.0.1", AgentVersion: ptr("v1")}))
	if !v1.IsNodeID(n.ID) || n.Status != v1.NodeStatusPending || n.ContainerdSock == "" || n.CreatedAt.IsZero() {
		t.Fatalf("created node: %+v", n)
	}
	if n.PublicIP != "203.0.113.1" || n.PrivateIP != "10.1.0.1" || *n.AgentVersion != "v1" {
		t.Fatalf("addresses: %+v", n)
	}
	if _, err := s.CreateNode(ctx, storage.CreateNodeParams{Name: "a"}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
	if got := must(s.GetNode(ctx, n.ID)); got.Name != "a" {
		t.Fatalf("GetNode: %+v", got)
	}
	if got := must(s.GetNodeByName(ctx, "a")); got.ID != n.ID {
		t.Fatalf("GetNodeByName: %+v", got)
	}
	for _, err := range []error{
		second(s.GetNode(ctx, "node_missing")),
		second(s.GetNodeByName(ctx, "missing")),
		second(s.GetNodeByAPIKeyHash(ctx, "missing")),
		second(s.SetNodeStatus(ctx, "node_missing", v1.NodeStatusReady)),
		second(s.HeartbeatNode(ctx, "node_missing")),
	} {
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("missing node error: %v", err)
		}
	}
	node(t, s, "b")
	if all := must(s.ListNodes(ctx)); len(all) != 2 {
		t.Fatalf("ListNodes: %d", len(all))
	}
	if sched := must(s.ListSchedulableNodes(ctx)); len(sched) != 0 {
		t.Fatalf("pending nodes are schedulable: %d", len(sched))
	}
	readyNode(t, s, "c", "10.0.3.0/24", "10.0.3.1")
	if sched := must(s.ListSchedulableNodes(ctx)); len(sched) != 1 || sched[0].Name != "c" {
		t.Fatalf("ListSchedulableNodes: %+v", sched)
	}

	hw := must(s.UpdateNodeHardware(ctx, n.ID, v1.NodeHardware{CpuCores: 2, MemoryTotalMiB: 1024, DiskTotalGiB: 9, Hostname: "h", OS: "linux", Arch: "amd64", KernelVersion: "6.1"}))
	if *hw.CpuCores != 2 || *hw.MemoryTotalMiB != 1024 || *hw.Hostname != "h" || *hw.KernelVersion != "6.1" || hw.HardwareReportedAt == nil {
		t.Fatalf("hardware: %+v", hw)
	}
}

func second[T any](_ T, err error) error { return err }

func testNodeStatus(t *testing.T, s storage.Interface) {
	n := node(t, s, "a")
	hb := must(s.HeartbeatNode(ctx, n.ID))
	if hb.LastHeartbeat == nil || hb.Status != v1.NodeStatusPending {
		t.Fatalf("heartbeat must stamp time and keep status: %+v", hb)
	}
	if got := must(s.SetNodeStatus(ctx, n.ID, v1.NodeStatusMaintenance)); got.Status != v1.NodeStatusMaintenance {
		t.Fatalf("SetNodeStatus: %s", got.Status)
	}
	if got := must(s.EvictNode(ctx, n.ID)); got.Status != v1.NodeStatusEvicted {
		t.Fatalf("EvictNode: %s", got.Status)
	}
}

func testStaleNodes(t *testing.T, s storage.Interface) {
	ready := readyNode(t, s, "ready", "10.0.1.0/24", "10.0.1.1")
	beat := readyNode(t, s, "beat", "10.0.2.0/24", "10.0.2.1")
	must(s.HeartbeatNode(ctx, beat.ID))
	pending := node(t, s, "pending")

	if stale := must(s.MarkStaleNodesNotReady(ctx, time.Now().Add(-time.Hour))); len(stale) != 0 {
		t.Fatalf("fresh nodes marked: %+v", stale)
	}
	stale := must(s.MarkStaleNodesNotReady(ctx, time.Now().Add(time.Hour)))
	if len(stale) != 2 {
		t.Fatalf("marked %d nodes, want the 2 ready ones", len(stale))
	}
	for _, n := range stale {
		if n.Status != v1.NodeStatusNotReady || (n.ID != ready.ID && n.ID != beat.ID) {
			t.Fatalf("marked %+v", n)
		}
	}
	if got := must(s.GetNode(ctx, pending.ID)); got.Status != v1.NodeStatusPending {
		t.Fatalf("pending node changed to %s", got.Status)
	}
	if again := must(s.MarkStaleNodesNotReady(ctx, time.Now().Add(time.Hour))); len(again) != 0 {
		t.Fatal("not_ready nodes marked twice")
	}
}

func testAPIKeys(t *testing.T, s storage.Interface) {
	n := node(t, s, "a")
	if err := s.SetNodeAPIKey(ctx, n.ID, "hash-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := must(s.GetNodeByAPIKeyHash(ctx, "hash-1")); got.ID != n.ID {
		t.Fatalf("lookup: %+v", got)
	}
	if err := s.SetNodeAPIKey(ctx, n.ID, "hash-2", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNodeByAPIKeyHash(ctx, "hash-1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("replaced key still valid: %v", err)
	}
	if err := s.SetNodeAPIKey(ctx, n.ID, "hash-3", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNodeByAPIKeyHash(ctx, "hash-3"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expired key valid: %v", err)
	}
	if err := s.SetNodeAPIKey(ctx, "node_missing", "x", time.Now()); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing node: %v", err)
	}
}

func testOverlay(t *testing.T, s storage.Interface) {
	if err := s.LockOverlayAllocation(ctx); err != nil {
		// Must be callable inside Tx; outside is allowed to fail.
		_ = err
	}
	err := s.Tx(ctx, func(tx storage.Interface) error { return tx.LockOverlayAllocation(ctx) })
	if err != nil {
		t.Fatalf("lock inside tx: %v", err)
	}
	a := readyNode(t, s, "a", "10.0.1.0/24", "10.0.1.1")
	b := readyNode(t, s, "b", "10.0.2.0/24", "10.0.2.1")
	if a.OverlaySubnet != "10.0.1.0/24" || a.OverlayIP != "10.0.1.1" || a.Status != v1.NodeStatusReady {
		t.Fatalf("SetNodeOverlay: %+v", a)
	}
	active := must(s.ListActiveOverlaySubnets(ctx))
	if len(active) != 2 {
		t.Fatalf("active subnets %v", active)
	}
	if r := must(s.ListReclaimableOverlaySubnets(ctx)); len(r) != 0 {
		t.Fatalf("reclaimable before eviction: %v", r)
	}
	must(s.EvictNode(ctx, b.ID))
	r := must(s.ListReclaimableOverlaySubnets(ctx))
	if len(r) != 1 || r[0].NodeID != b.ID || r[0].SubnetCIDR != "10.0.2.0/24" {
		t.Fatalf("reclaimable: %+v", r)
	}
	if active := must(s.ListActiveOverlaySubnets(ctx)); len(active) != 1 || active[0] != "10.0.1.0/24" {
		t.Fatalf("active after evict: %v", active)
	}
	if err := s.ClearNodeOverlay(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := must(s.GetNode(ctx, b.ID)); got.OverlaySubnet != "" || got.OverlayIP != "" {
		t.Fatalf("ClearNodeOverlay: %+v", got)
	}
	if r := must(s.ListReclaimableOverlaySubnets(ctx)); len(r) != 0 {
		t.Fatalf("reclaimable after clear: %v", r)
	}
}

func testOverlayPool(t *testing.T, s storage.Interface) {
	n := readyNode(t, s, "a", "10.0.1.0/24", "10.0.1.1")
	c1 := container(t, s, n.ID, "img", v1.CreateContainerRequest{})
	c2 := container(t, s, n.ID, "img", v1.CreateContainerRequest{})
	c3 := container(t, s, n.ID, "img", v1.CreateContainerRequest{})
	if err := s.SeedOverlayPool(ctx, n.ID, []string{"10.0.1.2", "10.0.1.3"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedOverlayPool(ctx, n.ID, []string{"10.0.1.3"}); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("duplicate seed: %v", err)
	}
	if free := must(s.CountFreeOverlayIPs(ctx, n.ID)); free != 2 {
		t.Fatalf("free %d", free)
	}
	ip1 := must(s.AllocateOverlayIP(ctx, n.ID, c1.ID))
	ip2 := must(s.AllocateOverlayIP(ctx, n.ID, c2.ID))
	if ip1 == ip2 {
		t.Fatalf("same ip twice: %s", ip1)
	}
	if _, err := s.AllocateOverlayIP(ctx, n.ID, c3.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("exhausted pool: %v", err)
	}
	used := must(s.ListOverlayIPs(ctx, n.ID, false))
	if len(used) != 2 || used[0].ContainerID == nil || used[0].AllocatedAt == nil {
		t.Fatalf("ListOverlayIPs: %+v", used)
	}
	if free := must(s.ListOverlayIPs(ctx, n.ID, true)); len(free) != 0 {
		t.Fatalf("free list: %+v", free)
	}
	if err := s.ReleaseOverlayIP(ctx, ip1); err != nil {
		t.Fatal(err)
	}
	free := must(s.ListOverlayIPs(ctx, n.ID, true))
	if len(free) != 1 || free[0].IP != ip1 || free[0].ContainerID != nil {
		t.Fatalf("after release: %+v", free)
	}
	if err := s.DeleteOverlayPool(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	if all := must(s.ListOverlayIPs(ctx, n.ID, false)); len(all) != 0 {
		t.Fatalf("pool after delete: %+v", all)
	}
}

func testJoinTokens(t *testing.T, s storage.Interface) {
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	single := must(s.CreateJoinToken(ctx, "h1", true, &exp))
	multi := must(s.CreateJoinToken(ctx, "h2", false, nil))
	if _, err := s.CreateJoinToken(ctx, "h1", true, nil); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("duplicate hash: %v", err)
	}
	got := must(s.GetJoinTokenByHash(ctx, "h1"))
	if got.ID != single.ID || !got.SingleUse || got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) || got.Token != "" {
		t.Fatalf("GetJoinTokenByHash: %+v", got)
	}
	if _, err := s.GetJoinTokenByHash(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if list := must(s.ListJoinTokens(ctx)); len(list) != 2 {
		t.Fatalf("list: %d", len(list))
	}

	n1, n2 := node(t, s, "n1"), node(t, s, "n2")
	if err := s.MarkJoinTokenUsed(ctx, single.ID, n1.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkJoinTokenUsed(ctx, single.ID, n2.ID); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("single use twice: %v", err)
	}
	used := must(s.GetJoinTokenByHash(ctx, "h1"))
	if used.UsedAt == nil || used.UsedBy == nil || *used.UsedBy != n1.ID {
		t.Fatalf("used: %+v", used)
	}
	for _, n := range []string{n1.ID, n2.ID} {
		if err := s.MarkJoinTokenUsed(ctx, multi.ID, n); err != nil {
			t.Fatalf("multi use: %v", err)
		}
	}
}

func testContainers(t *testing.T, s storage.Interface) {
	a := readyNode(t, s, "a", "10.0.1.0/24", "10.0.1.1")
	b := readyNode(t, s, "b", "10.0.2.0/24", "10.0.2.1")
	c := container(t, s, a.ID, "nginx", v1.CreateContainerRequest{
		Command: []string{"nginx", "-g", "daemon off;"}, Env: map[string]string{"K": "V"},
		WorkingDir: ptr("/srv"), CpuLimit: ptr(0.5), MemoryLimitMiB: ptr(int32(128)),
	})
	if c.Status != v1.ContainerStatusPending || c.RestartPolicy != v1.RestartPolicyOnFailure || c.CpuShares != 1024 ||
		c.ContainerdNs != "atellar" || c.Env["K"] != "V" || *c.WorkingDir != "/srv" || *c.CpuLimit != 0.5 ||
		*c.MemoryLimitMiB != 128 || len(c.Command) != 3 || c.NodeID != a.ID {
		t.Fatalf("created: %+v", c)
	}
	plain := container(t, s, b.ID, "redis", v1.CreateContainerRequest{RestartPolicy: v1.RestartPolicyAlways, CpuShares: ptr(int32(2))})
	if plain.Env == nil || plain.RestartPolicy != v1.RestartPolicyAlways || plain.CpuShares != 2 {
		t.Fatalf("defaults: %+v", plain)
	}

	sched := must(s.ScheduleContainer(ctx, c.ID))
	if sched.Status != v1.ContainerStatusScheduled || sched.ScheduledAt == nil {
		t.Fatalf("schedule: %+v", sched)
	}
	if _, err := s.GetContainer(ctx, "ctr_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := s.CreateContainer(ctx, storage.CreateContainerParams{NodeID: "node_missing", CreateContainerRequest: v1.CreateContainerRequest{Image: "x"}}); err == nil {
		t.Fatal("container on missing node accepted")
	}

	count := func(opts v1.ContainerListOptions) int { return len(must(s.ListContainers(ctx, opts))) }
	if count(v1.ContainerListOptions{}) != 2 || count(v1.ContainerListOptions{NodeID: a.ID}) != 1 || count(v1.ContainerListOptions{NodeID: "node_x"}) != 0 {
		t.Fatal("list filters")
	}
	removed := must(s.MarkContainerRemoved(ctx, c.ID))
	if removed.Status != v1.ContainerStatusRemoved {
		t.Fatalf("removed: %s", removed.Status)
	}
	if count(v1.ContainerListOptions{NodeID: a.ID, ActiveOnly: true}) != 1 {
		t.Fatal("removed containers still need the agent")
	}
	must(s.UpdateContainerRuntime(ctx, c.ID, storage.RuntimeUpdate{ContainerRuntimeReport: v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated}}))
	if count(v1.ContainerListOptions{NodeID: a.ID, ActiveOnly: true}) != 0 || count(v1.ContainerListOptions{ActiveOnly: true}) != 1 {
		t.Fatal("terminated containers must leave the active list")
	}
	if count(v1.ContainerListOptions{}) != 2 {
		t.Fatal("terminated containers stay in the full list")
	}
}

func testRuntimeUpdates(t *testing.T, s storage.Interface) {
	n := readyNode(t, s, "a", "10.0.1.0/24", "10.0.1.1")
	c := container(t, s, n.ID, "img", v1.CreateContainerRequest{})
	now := time.Now().UTC().Truncate(time.Microsecond)
	full := must(s.UpdateContainerRuntime(ctx, c.ID, storage.RuntimeUpdate{
		ContainerRuntimeReport: v1.ContainerRuntimeReport{
			ContainerdID: ptr("cd"), SnapshotKey: ptr("snap"), TaskPID: ptr(int32(9)), ImageDigest: ptr("sha256:1"),
			OverlayIP: "10.0.1.5", Status: v1.ContainerStatusRunning, ExitCode: ptr(int32(0)),
			ErrorMessage: ptr("old error"), RestartCount: ptr(int32(3)), LastFailedAt: &now,
		},
		StartedAt: &now,
	}))
	if *full.ContainerdID != "cd" || *full.SnapshotKey != "snap" || *full.TaskPID != 9 || *full.ImageDigest != "sha256:1" ||
		full.OverlayIP != "10.0.1.5" || full.RestartCount != 3 || full.StartedAt == nil || !full.LastFailedAt.Equal(now) {
		t.Fatalf("full update: %+v", full)
	}

	// A sparse report keeps identity fields and clears per-attempt ones.
	sparse := must(s.UpdateContainerRuntime(ctx, c.ID, storage.RuntimeUpdate{
		ContainerRuntimeReport: v1.ContainerRuntimeReport{Status: v1.ContainerStatusStopped},
		StoppedAt:              &now,
	}))
	if sparse.ContainerdID == nil || *sparse.ContainerdID != "cd" || sparse.OverlayIP != "10.0.1.5" || sparse.RestartCount != 3 ||
		sparse.ImageDigest == nil || sparse.StartedAt == nil || sparse.LastFailedAt == nil {
		t.Fatalf("sparse report wiped stored fields: %+v", sparse)
	}
	if sparse.TaskPID != nil || sparse.ExitCode != nil || sparse.ErrorMessage != nil || sparse.StoppedAt == nil {
		t.Fatalf("sparse report kept per-attempt fields: %+v", sparse)
	}
	if _, err := s.UpdateContainerRuntime(ctx, "ctr_missing", storage.RuntimeUpdate{ContainerRuntimeReport: v1.ContainerRuntimeReport{Status: v1.ContainerStatusRunning}}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func testResourceUsage(t *testing.T, s storage.Interface) {
	n := readyNode(t, s, "a", "10.0.1.0/24", "10.0.1.1")
	empty := must(s.NodeResourceUsage(ctx, n.ID))
	if empty.RunningCount != 0 || empty.TotalCPU != 0 || empty.TotalMemoryMiB != 0 {
		t.Fatalf("empty usage: %+v", empty)
	}
	container(t, s, n.ID, "a", v1.CreateContainerRequest{CpuLimit: ptr(1.5), MemoryLimitMiB: ptr(int32(256))})
	container(t, s, n.ID, "b", v1.CreateContainerRequest{CpuLimit: ptr(0.5)})
	gone := container(t, s, n.ID, "c", v1.CreateContainerRequest{CpuLimit: ptr(8.0), MemoryLimitMiB: ptr(int32(9999))})
	must(s.UpdateContainerRuntime(ctx, gone.ID, storage.RuntimeUpdate{ContainerRuntimeReport: v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated}}))

	u := must(s.NodeResourceUsage(ctx, n.ID))
	if u.RunningCount != 2 || u.TotalCPU != 2.0 || u.TotalMemoryMiB != 256 {
		t.Fatalf("usage: %+v", u)
	}
	if !must(s.HasActiveContainerWithImage(ctx, n.ID, "a")) {
		t.Fatal("active image not found")
	}
	if must(s.HasActiveContainerWithImage(ctx, n.ID, "c")) {
		t.Fatal("terminated container counted as active")
	}
}

func testEvents(t *testing.T, s storage.Interface) {
	n := readyNode(t, s, "a", "10.0.1.0/24", "10.0.1.1")
	c := container(t, s, n.ID, "img", v1.CreateContainerRequest{})
	if got := must(s.ListContainerEvents(ctx, c.ID)); len(got) != 0 {
		t.Fatalf("new container has events: %+v", got)
	}
	for _, e := range []v1.ContainerEventType{v1.ContainerEventScheduled, v1.ContainerEventStarted, v1.ContainerEventFailed} {
		if err := s.AddContainerEvent(ctx, v1.ContainerEvent{ContainerID: c.ID, NodeID: n.ID, Event: e, Message: ptr(string(e))}); err != nil {
			t.Fatalf("add %s: %v", e, err)
		}
	}
	got := must(s.ListContainerEvents(ctx, c.ID))
	if len(got) != 3 || got[0].Event != v1.ContainerEventFailed || got[2].Event != v1.ContainerEventScheduled {
		t.Fatalf("events must be newest first: %+v", got)
	}
	if got[0].ID == "" || got[0].CreatedAt.IsZero() || *got[0].Message != "failed" || got[0].NodeID != n.ID {
		t.Fatalf("event fields: %+v", got[0])
	}
	err := s.AddContainerEvent(ctx, v1.ContainerEvent{ContainerID: "ctr_missing", NodeID: n.ID, Event: v1.ContainerEventStarted})
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("event for missing container: %v", err)
	}
}

func testTx(t *testing.T, s storage.Interface) {
	boom := errors.New("boom")
	err := s.Tx(ctx, func(tx storage.Interface) error {
		if _, err := tx.CreateNode(ctx, storage.CreateNodeParams{Name: "rolled-back"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Tx error = %v", err)
	}
	if _, err := s.GetNodeByName(ctx, "rolled-back"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("rolled back node exists: %v", err)
	}

	err = s.Tx(ctx, func(tx storage.Interface) error {
		n, err := tx.CreateNode(ctx, storage.CreateNodeParams{Name: "kept"})
		if err != nil {
			return err
		}
		// Nested Tx joins the outer one.
		return tx.Tx(ctx, func(inner storage.Interface) error {
			_, err := inner.SetNodeStatus(ctx, n.ID, v1.NodeStatusReady)
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := must(s.GetNodeByName(ctx, "kept")); got.Status != v1.NodeStatusReady {
		t.Fatalf("committed node: %+v", got)
	}
}
