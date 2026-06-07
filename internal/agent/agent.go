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

	session, err := agentclient.NewSession(cfg, agentconfig.SystemConfigPath)
	if err != nil {
		return err
	}
	defer session.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("atellar agent started node_id=%s grpc=%s config=%s",
		cfg.NodeID, cfg.ResolveGrpcAddr(), agentconfig.SystemConfigPath)

	return session.Run(ctx, heartbeatEvery)
}
