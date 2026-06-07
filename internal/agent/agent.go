package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/internal/agent/grpcclient"
	"github.com/hasirciogluhq/atellar/internal/agent/hardware"
	"github.com/hasirciogluhq/atellar/internal/agent/overlay"
	"github.com/hasirciogluhq/atellar/internal/agent/runtime"
)

func Run() error {
	cfg, err := config.Load(config.SystemConfigPath)
	if err != nil {
		return err
	}

	heartbeatEvery, err := time.ParseDuration(cfg.HeartbeatInterval)
	if err != nil {
		return fmt.Errorf("invalid heartbeat_interval in config: %w", err)
	}

	session, err := grpcclient.NewSession(cfg, config.SystemConfigPath, nil)
	if err != nil {
		return err
	}
	defer session.Close()

	cpClient := runtime.NewCPClient(session.GRPCClient(), cfg.NodeAPIKey)

	netManager, err := overlay.NewManager(overlay.ManagerConfig{
		NodeID:            cfg.NodeID,
		BridgeName:        cfg.ResolveBridgeName(),
		OverlayIP:         cfg.OverlayIP,
		OverlaySubnet:     cfg.OverlaySubnet,
		ReconcileInterval: cfg.ResolveReconcileInterval(),
	}, session)
	if err != nil {
		return fmt.Errorf("overlay network manager: %w", err)
	}

	workloadManager := runtime.NewManager(
		cfg.NodeID,
		cfg.ContainerdSock,
		cfg.ResolveBridgeName(),
		cfg.OverlayIP,
		cfg.OverlaySubnet,
		session.GRPCClient(),
		cfg.NodeAPIKey,
	)

	session.SetNetworkReconciler(netManager)
	session.SetWorkloadTrigger(workloadManager)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go netManager.Run(ctx)
	go workloadManager.Run(ctx)

	hwReporter := hardware.NewReporter(session.GRPCClient(), cpClient.ReportHardware)
	go hwReporter.Run(ctx)

	log.Printf("atellar agent started node_id=%s grpc=%s bridge=%s config=%s",
		cfg.NodeID, cfg.ResolveGrpcAddr(), cfg.ResolveBridgeName(), config.SystemConfigPath)

	return session.Run(ctx, heartbeatEvery)
}
