package overlaynet

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type ManagerConfig struct {
	NodeID            string
	BridgeName        string
	OverlayIP         string
	OverlaySubnet     string
	ReconcileInterval time.Duration
}

type Manager struct {
	cfg       ManagerConfig
	state     *desiredState
	links     linkManager
	syncer    ClusterSyncer
	triggerCh chan struct{}
	reconcile sync.Mutex
}

func NewManager(cfg ManagerConfig, syncer ClusterSyncer) (*Manager, error) {
	if cfg.NodeID == "" {
		return nil, fmt.Errorf("node id is required")
	}

	if syncer == nil {
		return nil, fmt.Errorf("cluster syncer is required")
	}

	if cfg.BridgeName == "" {
		cfg.BridgeName = DefaultBridgeName
	}

	if cfg.ReconcileInterval <= 0 {
		cfg.ReconcileInterval = 30 * time.Second
	}

	local, err := parseLocalNode(cfg)
	if err != nil {
		return nil, err
	}

	return &Manager{
		cfg:       cfg,
		state:     newDesiredState(local),
		links:     newLinkManager(),
		syncer:    syncer,
		triggerCh: make(chan struct{}, 1),
	}, nil
}

func parseLocalNode(cfg ManagerConfig) (LocalNode, error) {
	local := LocalNode{
		NodeID:     cfg.NodeID,
		BridgeName: cfg.BridgeName,
	}

	if cfg.OverlayIP != "" {
		local.OverlayIP = net.ParseIP(cfg.OverlayIP)
	}

	if cfg.OverlaySubnet != "" {
		_, subnet, err := net.ParseCIDR(cfg.OverlaySubnet)
		if err != nil {
			return LocalNode{}, fmt.Errorf("invalid overlay_subnet: %w", err)
		}
		local.OverlaySubnet = subnet
	}

	return local, nil
}

func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.ReconcileInterval)
	defer ticker.Stop()

	m.syncAndReconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.syncAndReconcile(ctx)
		case <-m.triggerCh:
			m.reconcileOnce()
		}
	}
}

func (m *Manager) HandlePeerEvent(event PeerEvent) {
	m.state.applyPeerEvent(event)
	m.requestReconcile()
}

func (m *Manager) requestReconcile() {
	select {
	case m.triggerCh <- struct{}{}:
	default:
	}
}

func (m *Manager) syncAndReconcile(ctx context.Context) {
	if err := m.syncFromControlPlane(ctx); err != nil {
		log.Printf("overlaynet cluster sync failed: %v", err)
	}

	if err := m.Reconcile(); err != nil {
		log.Printf("overlaynet reconcile failed: %v", err)
	}
}

func (m *Manager) syncFromControlPlane(ctx context.Context) error {
	nodes, containers, err := m.syncer.SyncClusterState(ctx)
	if err != nil {
		return err
	}

	m.state.syncFromCluster(nodes, containers)
	return nil
}

func (m *Manager) reconcileOnce() {
	if err := m.Reconcile(); err != nil {
		log.Printf("overlaynet reconcile failed: %v", err)
	}
}

func (m *Manager) Reconcile() error {
	m.reconcile.Lock()
	defer m.reconcile.Unlock()

	local, _, _ := m.state.snapshot()
	if local.OverlayIP == nil || local.OverlaySubnet == nil {
		return fmt.Errorf("local overlay network is not configured")
	}

	if err := m.links.EnsureBridge(local.BridgeName); err != nil {
		return fmt.Errorf("ensure bridge: %w", err)
	}

	localAddr := &net.IPNet{
		IP:   local.OverlayIP,
		Mask: local.OverlaySubnet.Mask,
	}
	if err := m.links.EnsureAddress(local.BridgeName, localAddr); err != nil {
		return fmt.Errorf("ensure local address: %w", err)
	}

	desired := m.state.buildRoutes()
	current, err := m.links.ListRoutes(local.BridgeName)
	if err != nil {
		log.Printf("overlaynet list routes fallback to apply-only: %v", err)
		current = nil
	}

	desiredSet := routeSet(desired)
	currentSet := routeSet(current)

	for key, spec := range desiredSet {
		if err := m.links.EnsureRoute(spec); err != nil {
			log.Printf("overlaynet ensure route %s: %v", key, err)
		}
	}

	for key, spec := range currentSet {
		if _, ok := desiredSet[key]; ok {
			continue
		}

		if err := m.links.DeleteRoute(spec); err != nil {
			log.Printf("overlaynet delete stale route %s: %v", key, err)
		}
	}

	log.Printf(
		"overlaynet reconciled bridge=%s local=%s routes=%d",
		local.BridgeName,
		localAddr.String(),
		len(desiredSet),
	)

	return nil
}

func routeSet(routes []RouteSpec) map[string]RouteSpec {
	set := make(map[string]RouteSpec, len(routes))
	for _, route := range routes {
		if route.Destination == nil {
			continue
		}
		set[routeKey(route)] = route
	}
	return set
}

func routeKey(route RouteSpec) string {
	via := "direct"
	if route.Via != nil {
		via = route.Via.String()
	}
	return fmt.Sprintf("%s|%s|%s", route.Destination.String(), via, route.Device)
}
