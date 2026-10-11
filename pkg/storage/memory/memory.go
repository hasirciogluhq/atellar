// Package memory is an in-process storage.Interface for tests and local
// experiments. It follows the PostgreSQL store's semantics but keeps
// nothing on disk. Transactions are serialized and work on a copy that
// replaces the live state on success.
package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"maps"
	"net/netip"
	"slices"
	"sort"
	"sync"
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

type nodeRecord struct {
	node         v1.Node
	apiKeyHash   string
	apiKeyExpiry time.Time
}

type tokenRecord struct {
	token v1.JoinToken
	hash  string
}

type state struct {
	nodes      map[string]nodeRecord
	tokens     map[string]tokenRecord
	containers map[string]v1.Container
	events     map[string][]v1.ContainerEvent
	pool       map[string]v1.OverlayIP // keyed by IP
}

func newState() *state {
	return &state{
		nodes:      map[string]nodeRecord{},
		tokens:     map[string]tokenRecord{},
		containers: map[string]v1.Container{},
		events:     map[string][]v1.ContainerEvent{},
		pool:       map[string]v1.OverlayIP{},
	}
}

func (s *state) clone() *state {
	c := &state{
		nodes:      maps.Clone(s.nodes),
		tokens:     maps.Clone(s.tokens),
		containers: maps.Clone(s.containers),
		events:     maps.Clone(s.events),
		pool:       maps.Clone(s.pool),
	}
	return c
}

// Store is safe for concurrent use.
type Store struct {
	mu   *sync.Mutex
	txMu *sync.Mutex
	st   *state
	inTx bool
	now  func() time.Time
}

var _ storage.Interface = (*Store)(nil)

func New() *Store {
	return &Store{mu: &sync.Mutex{}, txMu: &sync.Mutex{}, st: newState(), now: time.Now}
}

// SetClock replaces time.Now, for tests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) Tx(ctx context.Context, fn func(storage.Interface) error) error {
	if s.inTx {
		return fn(s)
	}
	s.txMu.Lock()
	defer s.txMu.Unlock()

	s.mu.Lock()
	work := s.st.clone()
	s.mu.Unlock()

	tx := &Store{mu: &sync.Mutex{}, txMu: s.txMu, st: work, inTx: true, now: s.now}
	if err := fn(tx); err != nil {
		return err
	}

	s.mu.Lock()
	s.st = work
	s.mu.Unlock()
	return nil
}

func (s *Store) lock() func() {
	s.mu.Lock()
	return s.mu.Unlock
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (s *Store) GetNode(_ context.Context, id string) (*v1.Node, error) {
	defer s.lock()()
	rec, ok := s.st.nodes[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	n := rec.node
	return &n, nil
}

func (s *Store) GetNodeByName(_ context.Context, name string) (*v1.Node, error) {
	defer s.lock()()
	for _, rec := range s.st.nodes {
		if rec.node.Name == name {
			n := rec.node
			return &n, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *Store) GetNodeByAPIKeyHash(_ context.Context, hash string) (*v1.Node, error) {
	defer s.lock()()
	for _, rec := range s.st.nodes {
		if rec.apiKeyHash != "" && rec.apiKeyHash == hash && rec.apiKeyExpiry.After(s.now()) {
			n := rec.node
			return &n, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *Store) ListNodes(_ context.Context) ([]v1.Node, error) {
	defer s.lock()()
	return s.sortedNodes(func(v1.Node) bool { return true }, true), nil
}

func (s *Store) ListSchedulableNodes(_ context.Context) ([]v1.Node, error) {
	defer s.lock()()
	return s.sortedNodes(func(n v1.Node) bool {
		return n.Status == v1.NodeStatusReady && n.HardwareReportedAt != nil && n.CpuCores != nil && n.MemoryTotalMiB != nil
	}, false), nil
}

func (s *Store) sortedNodes(keep func(v1.Node) bool, newestFirst bool) []v1.Node {
	out := []v1.Node{}
	for _, rec := range s.st.nodes {
		if keep(rec.node) {
			out = append(out, rec.node)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if newestFirst {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (s *Store) CreateNode(_ context.Context, p storage.CreateNodeParams) (*v1.Node, error) {
	defer s.lock()()
	id := newID(v1.NodeIDPrefix)
	name := p.Name
	if name == "" {
		name = id
	}
	for _, rec := range s.st.nodes {
		if rec.node.Name == name {
			return nil, storage.ErrConflict
		}
	}
	sock := p.ContainerdSock
	if sock == "" {
		sock = "/run/containerd/containerd.sock"
	}
	now := s.now()
	n := v1.Node{
		ID: id, Name: name, PublicIP: p.PublicIP, PrivateIP: p.PrivateIP,
		Status: v1.NodeStatusPending, AgentVersion: p.AgentVersion, ContainerdSock: sock,
		CreatedAt: now, UpdatedAt: now,
	}
	s.st.nodes[id] = nodeRecord{node: n}
	return &n, nil
}

func (s *Store) updateNode(id string, fn func(*v1.Node)) (*v1.Node, error) {
	defer s.lock()()
	rec, ok := s.st.nodes[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	fn(&rec.node)
	s.st.nodes[id] = rec
	n := rec.node
	return &n, nil
}

func (s *Store) HeartbeatNode(_ context.Context, id string) (*v1.Node, error) {
	now := s.now()
	return s.updateNode(id, func(n *v1.Node) { n.LastHeartbeat = &now })
}

func (s *Store) SetNodeStatus(_ context.Context, id string, status v1.NodeStatus) (*v1.Node, error) {
	now := s.now()
	return s.updateNode(id, func(n *v1.Node) { n.Status, n.UpdatedAt = status, now })
}

func (s *Store) MarkStaleNodesNotReady(_ context.Context, staleBefore time.Time) ([]v1.Node, error) {
	defer s.lock()()
	now := s.now()
	out := []v1.Node{}
	for id, rec := range s.st.nodes {
		seen := rec.node.CreatedAt
		if rec.node.LastHeartbeat != nil {
			seen = *rec.node.LastHeartbeat
		}
		if rec.node.Status == v1.NodeStatusReady && seen.Before(staleBefore) {
			rec.node.Status, rec.node.UpdatedAt = v1.NodeStatusNotReady, now
			s.st.nodes[id] = rec
			out = append(out, rec.node)
		}
	}
	return out, nil
}

func (s *Store) EvictNode(_ context.Context, id string) (*v1.Node, error) {
	return s.SetNodeStatus(context.Background(), id, v1.NodeStatusEvicted)
}

func (s *Store) UpdateNodeHardware(_ context.Context, id string, hw v1.NodeHardware) (*v1.Node, error) {
	now := s.now()
	return s.updateNode(id, func(n *v1.Node) {
		n.CpuCores, n.MemoryTotalMiB, n.DiskTotalGiB = &hw.CpuCores, &hw.MemoryTotalMiB, &hw.DiskTotalGiB
		n.Hostname, n.OS, n.Arch, n.KernelVersion = &hw.Hostname, &hw.OS, &hw.Arch, &hw.KernelVersion
		n.HardwareReportedAt, n.UpdatedAt = &now, now
	})
}

func (s *Store) SetNodeAPIKey(_ context.Context, id, hash string, expiresAt time.Time) error {
	defer s.lock()()
	rec, ok := s.st.nodes[id]
	if !ok {
		return storage.ErrNotFound
	}
	rec.apiKeyHash, rec.apiKeyExpiry = hash, expiresAt
	s.st.nodes[id] = rec
	return nil
}

func (s *Store) LockOverlayAllocation(context.Context) error { return nil }

func (s *Store) SetNodeOverlay(_ context.Context, id, overlayIP, subnetCIDR string, status v1.NodeStatus) (*v1.Node, error) {
	if _, err := netip.ParsePrefix(subnetCIDR); err != nil {
		return nil, err
	}
	now := s.now()
	return s.updateNode(id, func(n *v1.Node) {
		n.OverlayIP, n.OverlaySubnet, n.Status, n.UpdatedAt = overlayIP, subnetCIDR, status, now
	})
}

func (s *Store) ClearNodeOverlay(_ context.Context, id string) error {
	_, err := s.updateNode(id, func(n *v1.Node) { n.OverlayIP, n.OverlaySubnet = "", "" })
	return err
}

func (s *Store) ListActiveOverlaySubnets(_ context.Context) ([]string, error) {
	defer s.lock()()
	out := []string{}
	for _, rec := range s.st.nodes {
		switch rec.node.Status {
		case v1.NodeStatusEvicted, v1.NodeStatusEvicting, v1.NodeStatusDown:
			continue
		}
		if rec.node.OverlaySubnet != "" {
			out = append(out, rec.node.OverlaySubnet)
		}
	}
	return out, nil
}

func (s *Store) ListReclaimableOverlaySubnets(_ context.Context) ([]storage.ReclaimableSubnet, error) {
	defer s.lock()()
	out := []storage.ReclaimableSubnet{}
	for _, rec := range s.st.nodes {
		if rec.node.Status == v1.NodeStatusEvicted && rec.node.OverlaySubnet != "" {
			out = append(out, storage.ReclaimableSubnet{NodeID: rec.node.ID, SubnetCIDR: rec.node.OverlaySubnet})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out, nil
}

func (s *Store) CreateJoinToken(_ context.Context, hash string, singleUse bool, expiresAt *time.Time) (*v1.JoinToken, error) {
	defer s.lock()()
	for _, rec := range s.st.tokens {
		if rec.hash == hash {
			return nil, storage.ErrConflict
		}
	}
	t := v1.JoinToken{ID: newID("jtok_"), SingleUse: singleUse, ExpiresAt: expiresAt, CreatedAt: s.now()}
	s.st.tokens[t.ID] = tokenRecord{token: t, hash: hash}
	return &t, nil
}

func (s *Store) GetJoinTokenByHash(_ context.Context, hash string) (*v1.JoinToken, error) {
	defer s.lock()()
	for _, rec := range s.st.tokens {
		if rec.hash == hash {
			t := rec.token
			return &t, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (s *Store) ListJoinTokens(_ context.Context) ([]v1.JoinToken, error) {
	defer s.lock()()
	out := []v1.JoinToken{}
	for _, rec := range s.st.tokens {
		out = append(out, rec.token)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) MarkJoinTokenUsed(_ context.Context, id, nodeID string) error {
	defer s.lock()()
	rec, ok := s.st.tokens[id]
	if !ok || (rec.token.SingleUse && rec.token.UsedAt != nil) {
		return storage.ErrConflict
	}
	now := s.now()
	rec.token.UsedAt, rec.token.UsedBy = &now, &nodeID
	s.st.tokens[id] = rec
	return nil
}

func (s *Store) CreateContainer(_ context.Context, p storage.CreateContainerParams) (*v1.Container, error) {
	defer s.lock()()
	if _, ok := s.st.nodes[p.NodeID]; !ok {
		return nil, storage.ErrNotFound
	}
	now := s.now()
	shares := int32(1024)
	if p.CpuShares != nil {
		shares = *p.CpuShares
	}
	policy := p.RestartPolicy
	if policy == "" {
		policy = v1.RestartPolicyOnFailure
	}
	env := p.Env
	if env == nil {
		env = map[string]string{}
	}
	c := v1.Container{
		ID: newID("ctr_"), NodeID: p.NodeID, ContainerdNs: "atellar", Image: p.Image,
		Command: p.Command, Entrypoint: p.Entrypoint, Env: env, WorkingDir: p.WorkingDir,
		CpuLimit: p.CpuLimit, CpuShares: shares, MemoryLimitMiB: p.MemoryLimitMiB,
		Status: v1.ContainerStatusPending, RestartPolicy: policy, CreatedAt: now, UpdatedAt: now,
	}
	s.st.containers[c.ID] = c
	return &c, nil
}

func (s *Store) GetContainer(_ context.Context, id string) (*v1.Container, error) {
	defer s.lock()()
	c, ok := s.st.containers[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &c, nil
}

func (s *Store) ListContainers(_ context.Context, opts v1.ContainerListOptions) ([]v1.Container, error) {
	defer s.lock()()
	out := []v1.Container{}
	for _, c := range s.st.containers {
		if opts.NodeID != "" && c.NodeID != opts.NodeID {
			continue
		}
		if opts.ActiveOnly && c.Status == v1.ContainerStatusTerminated {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) updateContainer(id string, fn func(*v1.Container)) (*v1.Container, error) {
	defer s.lock()()
	c, ok := s.st.containers[id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	fn(&c)
	c.UpdatedAt = s.now()
	s.st.containers[id] = c
	return &c, nil
}

func (s *Store) ScheduleContainer(_ context.Context, id string) (*v1.Container, error) {
	now := s.now()
	return s.updateContainer(id, func(c *v1.Container) { c.Status, c.ScheduledAt = v1.ContainerStatusScheduled, &now })
}

func (s *Store) MarkContainerRemoved(_ context.Context, id string) (*v1.Container, error) {
	return s.updateContainer(id, func(c *v1.Container) { c.Status = v1.ContainerStatusRemoved })
}

func (s *Store) UpdateContainerRuntime(_ context.Context, id string, u storage.RuntimeUpdate) (*v1.Container, error) {
	return s.updateContainer(id, func(c *v1.Container) {
		keep(&c.ContainerdID, u.ContainerdID)
		keep(&c.SnapshotKey, u.SnapshotKey)
		keep(&c.ImageDigest, u.ImageDigest)
		keep(&c.LastFailedAt, u.LastFailedAt)
		keep(&c.ScheduledAt, u.ScheduledAt)
		keep(&c.StartedAt, u.StartedAt)
		keep(&c.StoppedAt, u.StoppedAt)
		if u.OverlayIP != "" {
			c.OverlayIP = u.OverlayIP
		}
		if u.RestartCount != nil {
			c.RestartCount = *u.RestartCount
		}
		c.TaskPID, c.ExitCode, c.ErrorMessage, c.Status = u.TaskPID, u.ExitCode, u.ErrorMessage, u.Status
	})
}

func keep[T any](dst **T, src *T) {
	if src != nil {
		*dst = src
	}
}

func (s *Store) NodeResourceUsage(_ context.Context, nodeID string) (*storage.ResourceUsage, error) {
	defer s.lock()()
	u := &storage.ResourceUsage{}
	for _, c := range s.st.containers {
		if c.NodeID != nodeID || !c.Status.Active() {
			continue
		}
		u.RunningCount++
		if c.CpuLimit != nil {
			u.TotalCPU += *c.CpuLimit
		}
		if c.MemoryLimitMiB != nil {
			u.TotalMemoryMiB += *c.MemoryLimitMiB
		}
	}
	return u, nil
}

func (s *Store) HasActiveContainerWithImage(_ context.Context, nodeID, image string) (bool, error) {
	defer s.lock()()
	for _, c := range s.st.containers {
		if c.NodeID == nodeID && c.Image == image && c.Status.Active() {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) AddContainerEvent(_ context.Context, ev v1.ContainerEvent) error {
	defer s.lock()()
	if _, ok := s.st.containers[ev.ContainerID]; !ok {
		return storage.ErrNotFound
	}
	ev.ID, ev.CreatedAt = newID("evt_"), s.now()
	s.st.events[ev.ContainerID] = append(s.st.events[ev.ContainerID], ev)
	return nil
}

func (s *Store) ListContainerEvents(_ context.Context, id string) ([]v1.ContainerEvent, error) {
	defer s.lock()()
	out := slices.Clone(s.st.events[id])
	slices.Reverse(out)
	if out == nil {
		out = []v1.ContainerEvent{}
	}
	return out, nil
}

func (s *Store) SeedOverlayPool(_ context.Context, nodeID string, ips []string) error {
	defer s.lock()()
	for _, ip := range ips {
		if _, exists := s.st.pool[ip]; exists {
			return storage.ErrConflict
		}
		s.st.pool[ip] = v1.OverlayIP{IP: ip, NodeID: nodeID}
	}
	return nil
}

func (s *Store) DeleteOverlayPool(_ context.Context, nodeID string) error {
	defer s.lock()()
	for ip, e := range s.st.pool {
		if e.NodeID == nodeID {
			delete(s.st.pool, ip)
		}
	}
	return nil
}

func (s *Store) poolSorted(nodeID string, freeOnly bool) []v1.OverlayIP {
	out := []v1.OverlayIP{}
	for _, e := range s.st.pool {
		if e.NodeID == nodeID && (!freeOnly || e.ContainerID == nil) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return netip.MustParseAddr(out[i].IP).Less(netip.MustParseAddr(out[j].IP))
	})
	return out
}

func (s *Store) ListOverlayIPs(_ context.Context, nodeID string, freeOnly bool) ([]v1.OverlayIP, error) {
	defer s.lock()()
	return s.poolSorted(nodeID, freeOnly), nil
}

func (s *Store) CountFreeOverlayIPs(_ context.Context, nodeID string) (int, error) {
	defer s.lock()()
	return len(s.poolSorted(nodeID, true)), nil
}

func (s *Store) AllocateOverlayIP(_ context.Context, nodeID, containerID string) (string, error) {
	defer s.lock()()
	free := s.poolSorted(nodeID, true)
	if len(free) == 0 {
		return "", storage.ErrNotFound
	}
	e := free[0]
	now := s.now()
	e.ContainerID, e.AllocatedAt = &containerID, &now
	s.st.pool[e.IP] = e
	return e.IP, nil
}

func (s *Store) ReleaseOverlayIP(_ context.Context, ip string) error {
	defer s.lock()()
	if e, ok := s.st.pool[ip]; ok {
		e.ContainerID, e.AllocatedAt = nil, nil
		s.st.pool[ip] = e
	}
	return nil
}
