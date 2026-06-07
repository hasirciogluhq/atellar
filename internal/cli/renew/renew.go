package renew

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/internal/client/controlplane"
)

type Options struct {
	ControlPlaneURL string
	NodeAPIKey      string
	ConfigPath      string
	UpdateConfig    bool
}

type Result struct {
	NodeAPIKey      string
	APIKeyExpiresAt string
	ConfigPath      string
}

func Execute(ctx context.Context, opts Options) (*Result, error) {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = config.SystemConfigPath
	}

	controlPlaneURL := opts.ControlPlaneURL
	apiKey := opts.NodeAPIKey

	if opts.UpdateConfig || (controlPlaneURL == "" && apiKey == "") {
		cfg, err := config.Load(configPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}

		if controlPlaneURL == "" {
			controlPlaneURL = cfg.ControlPlaneURL
		}

		if apiKey == "" {
			apiKey = cfg.NodeAPIKey
		}
	}

	if controlPlaneURL == "" {
		return nil, fmt.Errorf("control plane url is required")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("node api key is required")
	}

	client := controlplane.NewClient(controlPlaneURL)
	renewed, err := client.RenewNodeAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	if opts.UpdateConfig || opts.ConfigPath != "" {
		if err := config.UpdateNodeAPIKey(configPath, renewed.NodeAPIKey, renewed.APIKeyExpiresAt); err != nil {
			return nil, fmt.Errorf("update config: %w", err)
		}
	}

	return &Result{
		NodeAPIKey:      renewed.NodeAPIKey,
		APIKeyExpiresAt: renewed.APIKeyExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		ConfigPath:      configPath,
	}, nil
}
