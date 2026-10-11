// Package agent is the node agent: it keeps the node registered and alive,
// builds the overlay network and runs the node's containers.
package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/workload"
	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/client/informer"
	"github.com/hasirciogluhq/atellar/pkg/config/agentconfig"
	"github.com/hasirciogluhq/atellar/pkg/network/overlay"
	"github.com/hasirciogluhq/atellar/pkg/node/hardware"
	"github.com/hasirciogluhq/atellar/pkg/runtime/containerd"
	"github.com/hasirciogluhq/atellar/pkg/version"
)

const (
	hardwareInterval = 10 * time.Minute
	keyCheckInterval = time.Hour
)

// Agent holds the running node agent.
type Agent struct {
	configPath string

	mu  sync.RWMutex
	cfg agentconfig.Config
}

func New(configPath string, cfg *agentconfig.Config) *Agent {
	return &Agent{configPath: configPath, cfg: *cfg}
}

func (a *Agent) apiKey() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.NodeAPIKey
}

// Run starts every loop and blocks until ctx is done.
func (a *Agent) Run(ctx context.Context) error {
	cfg := a.cfg
	c, err := client.New(client.Config{ControlPlane: cfg.ControlPlane, Transport: cfg.Transport, Token: a.apiKey})
	if err != nil {
		return err
	}
	defer c.Close()

	net := cfg.Network
	ovl, err := overlay.NewManager(overlay.Config{
		NodeID:            cfg.NodeID,
		UnderlayIP:        net.UnderlayIP,
		BridgeName:        net.BridgeName,
		VXLANName:         net.VXLANName,
		VNI:               net.VNI,
		Port:              net.Port,
		MTU:               net.MTU,
		ReconcileInterval: time.Duration(net.ReconcileInterval),
	}, nil)
	if err != nil {
		return err
	}
	ovlCfg := ovl.Config()

	nodes := informer.New(informer.Config[v1.Node]{
		Name:     "nodes",
		Open:     func(ctx context.Context) (watch.Interface[v1.Node], error) { return c.Nodes().Watch(ctx) },
		Key:      func(n v1.Node) string { return n.ID },
		OnChange: ovl.SetNodes,
	})
	local := func() (string, string) {
		if self, ok := nodes.Get(cfg.NodeID); ok {
			return self.OverlayIP, self.OverlaySubnet
		}
		return "", ""
	}

	workloads := workload.NewManager(workload.Config{
		NodeID:     cfg.NodeID,
		BridgeName: ovlCfg.BridgeName,
		MTU:        ovlCfg.MTU,
		Local:      local,
	}, c.Containers(), containerd.New(cfg.ContainerdSock, cfg.LogDir), workload.Netns{})

	// The container watch only wakes the workload manager, which then
	// lists fresh state, so a missed or reordered event cannot leave it
	// acting on stale data.
	containers := informer.New(informer.Config[v1.Container]{
		Name: "containers",
		Open: func(ctx context.Context) (watch.Interface[v1.Container], error) {
			return c.Containers().Watch(ctx, v1.ContainerListOptions{NodeID: cfg.NodeID})
		},
		Key:      func(c v1.Container) string { return c.ID },
		OnChange: func([]v1.Container) { workloads.Trigger() },
	})

	log.Printf("atellar agent %s: node %s (%s), control plane %s over %s",
		version.Version, cfg.NodeName, cfg.NodeID, cfg.ControlPlane.Address, cfg.Transport)

	var wg sync.WaitGroup
	for _, run := range []func(context.Context){
		nodes.Run,
		containers.Run,
		ovl.Run,
		workloads.Run,
		func(ctx context.Context) { a.heartbeat(ctx, c.Nodes()) },
		func(ctx context.Context) { a.reportHardware(ctx, c.Nodes()) },
		func(ctx context.Context) { a.renewKey(ctx, c.Nodes()) },
	} {
		wg.Add(1)
		go func() { defer wg.Done(); run(ctx) }()
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (a *Agent) heartbeat(ctx context.Context, nodes api.NodeInterface) {
	every := time.Duration(a.cfg.HeartbeatInterval)
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	failing := false
	for {
		err := nodes.SelfHeartbeat(ctx)
		switch {
		case err != nil && ctx.Err() == nil && !failing:
			log.Printf("heartbeat: %v", err)
			failing = true
		case err == nil && failing:
			log.Printf("heartbeat: recovered")
			failing = false
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Agent) reportHardware(ctx context.Context, nodes api.NodeInterface) {
	ticker := time.NewTicker(hardwareInterval)
	defer ticker.Stop()
	for {
		if info, err := hardware.Collect(); err != nil {
			log.Printf("hardware: collect: %v", err)
		} else if err := nodes.SelfReportHardware(ctx, v1.NodeHardware{
			CpuCores: info.CpuCores, MemoryTotalMiB: info.MemoryTotalMiB, DiskTotalGiB: info.DiskTotalGiB,
			Hostname: info.Hostname, OS: info.OS, Arch: info.Arch, KernelVersion: info.KernelVersion,
		}); err != nil && ctx.Err() == nil {
			log.Printf("hardware: report: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// renewKey replaces the API key before it expires and saves it, so the
// agent survives restarts with the new key.
func (a *Agent) renewKey(ctx context.Context, nodes api.NodeInterface) {
	ticker := time.NewTicker(keyCheckInterval)
	defer ticker.Stop()
	for {
		if err := a.renewIfDue(ctx, nodes); err != nil && ctx.Err() == nil {
			log.Printf("api key: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *Agent) renewIfDue(ctx context.Context, nodes api.NodeInterface) error {
	a.mu.RLock()
	expires := a.cfg.APIKeyExpiresAt
	a.mu.RUnlock()
	if !token.ShouldRenewNodeAPIKey(expires) {
		return nil
	}
	key, err := nodes.SelfRenewAPIKey(ctx)
	if err != nil {
		return fmt.Errorf("renew: %w", err)
	}
	a.mu.Lock()
	a.cfg.NodeAPIKey, a.cfg.APIKeyExpiresAt = key.NodeAPIKey, key.APIKeyExpiresAt
	snapshot := a.cfg
	a.mu.Unlock()
	if err := agentconfig.Save(a.configPath, &snapshot); err != nil {
		// The new key is live; losing it on restart would lock the node out.
		return fmt.Errorf("renewed, but saving %s failed: %w", a.configPath, err)
	}
	log.Printf("api key: renewed, expires %s", key.APIKeyExpiresAt.Format(time.RFC3339))
	return nil
}
