package join

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/agent"
	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/internal/client/controlplane"
)

type Options struct {
	Token             string
	ControlPlaneURL   string
	NodeName          string
	PublicIP          string
	PrivateIP         string
	ContainerdSock    string
	HeartbeatInterval string
	ConfigPath        string
}

func Execute(ctx context.Context, opts Options) (*config.Config, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("join token is required")
	}

	controlPlaneURL := opts.ControlPlaneURL
	if controlPlaneURL == "" {
		controlPlaneURL = "http://localhost:8080"
	}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = config.SystemConfigPath
	}

	containerdSock := opts.ContainerdSock
	if containerdSock == "" {
		containerdSock = "/run/containerd/containerd.sock"
	}

	heartbeatInterval := opts.HeartbeatInterval
	if heartbeatInterval == "" {
		heartbeatInterval = "5s"
	}

	client := controlplane.NewClient(controlPlaneURL)
	result, err := client.Register(ctx, opts.Token, controlplane.RegisterRequest{
		Name:           opts.NodeName,
		PublicIP:       opts.PublicIP,
		PrivateIP:      opts.PrivateIP,
		AgentVersion:   agent.Version,
		ContainerdSock: containerdSock,
	})
	if err != nil {
		return nil, err
	}

	cfg := config.Config{
		ControlPlaneURL:   controlPlaneURL,
		NodeID:            result.Node.ID,
		NodeName:          result.Node.Name,
		OverlayIP:         result.Node.OverlayIP.String(),
		OverlaySubnet:     result.Node.OverlaySubnet,
		NodeAPIKey:        result.NodeAPIKey,
		APIKeyExpiresAt:   result.APIKeyExpiresAt,
		ContainerdSock:    containerdSock,
		HeartbeatInterval: heartbeatInterval,
	}

	if err := config.Save(configPath, cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
