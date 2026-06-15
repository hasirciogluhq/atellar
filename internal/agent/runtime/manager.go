package runtime

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hasirciogluhq/atellar/internal/agent/netns"
	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
)

const pollInterval = 15 * time.Second

type Manager struct {
	nodeID            string
	nodeOverlayIP     string
	nodeOverlaySubnet string
	cp                *CPClient
	containerd        *ContainerdRuntime
	bridgeName        string

	triggerCh chan struct{}
	mu        sync.Mutex
	local     map[string]LocalContainer
}

func NewManager(
	nodeID, containerdSocket, bridgeName, nodeOverlayIP, nodeOverlaySubnet string,
	grpcClient atellarv1.AgentServiceClient,
	apiKeyProvider func() string,
) *Manager {
	return &Manager{
		nodeID:            nodeID,
		nodeOverlayIP:     nodeOverlayIP,
		nodeOverlaySubnet: nodeOverlaySubnet,
		cp:                NewCPClient(grpcClient, apiKeyProvider),
		containerd:        NewContainerdRuntime(containerdSocket, bridgeName, "/var/log/atellar"),
		bridgeName:        bridgeName,
		triggerCh:         make(chan struct{}, 1),
		local:             make(map[string]LocalContainer),
	}
}

func (m *Manager) Trigger() {
	select {
	case m.triggerCh <- struct{}{}:
	default:
	}
}

func (m *Manager) Run(ctx context.Context) {
	go m.reconcileLoop(ctx)
	_ = m.containerd.SubscribeExits(ctx, m.onTaskExit)
}

func (m *Manager) reconcileLoop(ctx context.Context) {
	m.reconcile(ctx)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reconcile(ctx)
		case <-m.triggerCh:
			m.reconcile(ctx)
		}
	}
}

func (m *Manager) reconcile(ctx context.Context) {
	workloads, err := m.cp.ListWorkloads(ctx)
	if err != nil {
		log.Printf("workload poll failed: %v", err)
		return
	}

	active := make(map[string]struct{}, len(workloads))
	for _, w := range workloads {
		active[w.ID] = struct{}{}
		if w.ContainerdNs == "" {
			w.ContainerdNs = "atellar"
		}
		m.reconcileOne(ctx, w)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.local {
		if _, ok := active[id]; !ok {
			m.cleanupLocal(ctx, id)
		}
	}
}

func (m *Manager) reconcileOne(ctx context.Context, w Workload) {
	switch w.Status {
	case "removed", "terminated":
		m.cleanupLocal(ctx, w.ID)
		return
	case "failed":
		if !shouldRetry(w.LastFailedAtUnix, w.RestartCount) {
			return
		}
		fallthrough
	case "backoff":
		if w.Status == "backoff" && !shouldRetry(w.LastFailedAtUnix, w.RestartCount) {
			return
		}
		fallthrough
	case "pending", "scheduled", "stopped", "crashed":
		m.runPipeline(ctx, w)
	case "running":
		running, pid, err := m.containerd.TaskRunning(ctx, w.ID)
		if err != nil {
			return
		}
		if !running {
			m.runPipeline(ctx, w)
			return
		}
		m.mu.Lock()
		lc := m.local[w.ID]
		lc.TaskPID = pid
		m.local[w.ID] = lc
		m.mu.Unlock()
	}
}

func (m *Manager) runPipeline(ctx context.Context, w Workload) {
	if err := m.report(ctx, w.ID, RuntimeReport{ContainerID: w.ID, Status: "pulling", RestartCount: w.RestartCount}); err != nil {
		log.Printf("report pulling failed container=%s: %v", w.ID, err)
		return
	}

	if _, err := m.containerd.EnsureImage(ctx, w.Image, w.ImageDigest); err != nil {
		m.fail(ctx, w, err)
		return
	}

	if err := m.report(ctx, w.ID, RuntimeReport{ContainerID: w.ID, Status: "creating", RestartCount: w.RestartCount}); err != nil {
		m.fail(ctx, w, err)
		return
	}

	overlayIP, err := m.cp.AllocateOverlayIP(ctx, w.ID)
	if err != nil {
		m.fail(ctx, w, err)
		return
	}

	if err := netns.Setup(netns.Config{
		ContainerID:       w.ID,
		OverlayIP:         overlayIP,
		NodeOverlayIP:     m.nodeOverlayIP,
		NodeOverlaySubnet: m.nodeOverlaySubnet,
		BridgeName:        m.bridgeName,
	}); err != nil {
		m.fail(ctx, w, err)
		return
	}

	created, err := m.containerd.RunContainer(ctx, w, overlayIP)
	if err != nil {
		m.fail(ctx, w, err)
		return
	}

	m.mu.Lock()
	m.local[w.ID] = *created
	m.mu.Unlock()

	_ = m.report(ctx, w.ID, RuntimeReport{
		ContainerID:  w.ID,
		ContainerdID: created.ContainerdID,
		SnapshotKey:  created.SnapshotKey,
		TaskPID:      created.TaskPID,
		ImageDigest:  created.ImageDigest,
		OverlayIP:    overlayIP,
		Status:       "running",
		RestartCount: w.RestartCount,
	})
}

func (m *Manager) fail(ctx context.Context, w Workload, err error) {
	netns.Teardown(w.ID)
	_ = m.containerd.PurgeState(ctx, w.ID)

	now := NowUnix()
	restartCount := w.RestartCount + 1
	status := "backoff"
	if !shouldRetry(now, restartCount) {
		status = "failed"
	}
	_ = m.report(ctx, w.ID, RuntimeReport{
		ContainerID:      w.ID,
		Status:           status,
		ErrorMessage:     err.Error(),
		RestartCount:     restartCount,
		LastFailedAtUnix: now,
	})
}

func (m *Manager) cleanupLocal(ctx context.Context, containerID string) {
	netns.Teardown(containerID)
	_ = m.containerd.StopAndDelete(ctx, containerID)
	m.mu.Lock()
	delete(m.local, containerID)
	m.mu.Unlock()
	_ = m.report(ctx, containerID, RuntimeReport{
		ContainerID: containerID,
		Status:      "terminated",
	})
}

func (m *Manager) onTaskExit(containerID string, exitCode int) {
	status := "stopped"
	if exitCode != 0 {
		status = "crashed"
	}
	ctx := context.Background()
	m.mu.Lock()
	lc, ok := m.local[containerID]
	m.mu.Unlock()

	report := RuntimeReport{
		ContainerID: containerID,
		Status:      status,
		ExitCode:    int32(exitCode),
	}
	if ok {
		report.ContainerdID = lc.ContainerdID
		report.SnapshotKey = lc.SnapshotKey
		report.ImageDigest = lc.ImageDigest
		report.OverlayIP = lc.OverlayIP
	}

	if err := m.cp.ReportRuntime(ctx, report); err != nil {
		log.Printf("exit report failed container=%s: %v", containerID, err)
	}

	if status == "crashed" {
		m.Trigger()
	}
}

func (m *Manager) report(ctx context.Context, containerID string, report RuntimeReport) error {
	if report.ContainerID == "" {
		report.ContainerID = containerID
	}
	if err := m.cp.ReportRuntime(ctx, report); err != nil {
		return fmt.Errorf("report runtime: %w", err)
	}
	return nil
}
