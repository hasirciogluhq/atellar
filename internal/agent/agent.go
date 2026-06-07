package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hasirciogluhq/atellar/internal/pkg/agentconfig"
	"github.com/hasirciogluhq/atellar/internal/pkg/controlplane"
)

type Agent struct {
	cfg            *agentconfig.Config
	controlPlane   *controlplane.Client
	heartbeatEvery time.Duration
}

func New(cfg *agentconfig.Config) (*Agent, error) {
	heartbeatEvery, err := time.ParseDuration(cfg.HeartbeatInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid heartbeat_interval in config: %w", err)
	}

	return &Agent{
		cfg:            cfg,
		controlPlane:   controlplane.NewClient(cfg.ControlPlaneURL),
		heartbeatEvery: heartbeatEvery,
	}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	log.Printf("atellar agent started node_id=%s config=%s heartbeat=%s",
		a.cfg.NodeID, agentconfig.SystemConfigPath, a.heartbeatEvery)

	ticker := time.NewTicker(a.heartbeatEvery)
	defer ticker.Stop()

	if err := a.sendHeartbeat(ctx); err != nil {
		log.Printf("initial heartbeat failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.sendHeartbeat(ctx); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		}
	}
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	return a.controlPlane.SendHeartbeat(ctx, a.cfg.NodeID)
}

func Run() error {
	cfg, err := agentconfig.Load(agentconfig.SystemConfigPath)
	if err != nil {
		return err
	}

	agent, err := New(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return agent.Run(ctx)
}
