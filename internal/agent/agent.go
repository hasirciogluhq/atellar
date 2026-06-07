package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hasirciogluhq/atellar/internal/grpc/agentclient"
	"github.com/hasirciogluhq/atellar/internal/pkg/agentconfig"
	"github.com/hasirciogluhq/atellar/internal/pkg/overlaynet"
)

func Run() error {
	cfg, err := agentconfig.Load(agentconfig.SystemConfigPath)
	if err != nil {
		return err
	}

	heartbeatEvery, err := time.ParseDuration(cfg.HeartbeatInterval)
	if err != nil {
		return fmt.Errorf("invalid heartbeat_interval in config: %w", err)
	}

	session, err := agentclient.NewSession(cfg, agentconfig.SystemConfigPath, nil)
	if err != nil {
		return err
	}
	defer session.Close()

	netManager, err := overlaynet.NewManager(overlaynet.ManagerConfig{
		NodeID:            cfg.NodeID,
		BridgeName:        cfg.ResolveBridgeName(),
		OverlayIP:         cfg.OverlayIP,
		OverlaySubnet:     cfg.OverlaySubnet,
		ReconcileInterval: cfg.ResolveReconcileInterval(),
	}, session)
	if err != nil {
		return fmt.Errorf("overlay network manager: %w", err)
	}

	session.SetNetworkReconciler(netManager)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go netManager.Run(ctx)

	log.Printf("atellar agent started node_id=%s grpc=%s bridge=%s config=%s",
		cfg.NodeID, cfg.ResolveGrpcAddr(), cfg.ResolveBridgeName(), agentconfig.SystemConfigPath)

	return session.Run(ctx, heartbeatEvery)
}
