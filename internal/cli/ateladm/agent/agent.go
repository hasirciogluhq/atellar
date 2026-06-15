package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hasirciogluhq/atellar/internal/agent"
	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/pkg/client"
)

const (
	AgentBinPath    = "/usr/local/bin/atelagent"
	SystemdUnitPath = "/etc/systemd/system/atellar-agent.service"
	LogDir          = "/var/log/atellar"
)

type JoinOptions struct {
	JoinToken         string
	ControlPlane      client.ControlPlane
	NodeName          string
	PublicIP          string
	PrivateIP         string
	ContainerdSocket  string
	HeartbeatInterval string
}

func ValidateJoinOptions(opts JoinOptions) error {
	if opts.JoinToken == "" {
		return fmt.Errorf("--join-token is required")
	}
	if opts.NodeName == "" {
		return fmt.Errorf("--name is required")
	}
	if opts.PublicIP == "" {
		return fmt.Errorf("--public-ip is required")
	}
	if opts.PrivateIP == "" {
		return fmt.Errorf("--private-ip is required")
	}
	if err := opts.ControlPlane.Validate(); err != nil {
		return fmt.Errorf("--control-plane-address, --http-port, --grpc-port: %w", err)
	}
	return nil
}

func Join(ctx context.Context, opts JoinOptions) (*config.Config, error) {
	if err := ValidateJoinOptions(opts); err != nil {
		return nil, err
	}

	containerdSocket := opts.ContainerdSocket
	if containerdSocket == "" {
		containerdSocket = "/run/containerd/containerd.sock"
	}

	interval := opts.HeartbeatInterval
	if interval == "" {
		interval = "5s"
	}

	if _, err := os.Stat(containerdSocket); err != nil {
		return nil, fmt.Errorf("containerd socket not found at %s", containerdSocket)
	}

	if err := os.MkdirAll(config.SystemConfigDir, 0o755); err != nil {
		return nil, err
	}

	api := client.New(client.Options{BaseURL: opts.ControlPlane.HTTPBaseURL()})
	result, err := api.RegisterNode(ctx, opts.JoinToken, client.RegisterNodeRequest{
		Name:           opts.NodeName,
		PublicIP:       opts.PublicIP,
		PrivateIP:      opts.PrivateIP,
		AgentVersion:   agent.Version,
		ContainerdSock: containerdSocket,
	})
	if err != nil {
		return nil, err
	}

	cfg := config.Config{
		ControlPlaneAddress: opts.ControlPlane.Address,
		HTTPPort:            opts.ControlPlane.HTTPPort,
		GRPCPort:            opts.ControlPlane.GRPCPort,
		NodeID:              result.Node.ID,
		NodeName:            result.Node.Name,
		OverlayIP:           result.Node.OverlayIP,
		OverlaySubnet:       result.Node.OverlaySubnet,
		NodeAPIKey:          result.NodeAPIKey,
		APIKeyExpiresAt:     result.APIKeyExpiresAt,
		ContainerdSock:      containerdSocket,
		HeartbeatInterval:   interval,
	}

	if err := config.Save(config.SystemConfigPath, cfg); err != nil {
		return nil, err
	}

	_ = runSystemctl("restart", "atellar-agent.service")
	return &cfg, nil
}

type InstallResult struct {
	UnitPath string
	NodeID   string
}

func Install(ctx context.Context, autoJoin bool, join JoinOptions) (*InstallResult, error) {
	if autoJoin {
		if err := ValidateJoinOptions(join); err != nil {
			return nil, fmt.Errorf("auto-join: %w", err)
		}
	}

	for _, dir := range []string{config.SystemConfigDir, LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	if _, err := os.Stat(AgentBinPath); err != nil {
		return nil, fmt.Errorf("atelagent not found at %s", AgentBinPath)
	}

	if err := os.WriteFile(SystemdUnitPath, []byte(systemdUnit(AgentBinPath)), 0o644); err != nil {
		return nil, fmt.Errorf("write systemd unit: %w", err)
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return nil, err
	}
	if err := runSystemctl("enable", "atellar-agent.service"); err != nil {
		return nil, err
	}

	result := &InstallResult{UnitPath: SystemdUnitPath}

	if autoJoin {
		cfg, err := Join(ctx, join)
		if err != nil {
			return nil, err
		}
		result.NodeID = cfg.NodeID
	}

	return result, nil
}

type RenewKeyResult struct {
	NodeAPIKey      string
	APIKeyExpiresAt string
}

func RenewKey(ctx context.Context) (*RenewKeyResult, error) {
	cfg, err := config.Load(config.SystemConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	api := client.New(client.Options{BaseURL: cfg.HTTPBaseURL()})
	renewed, err := api.RenewNodeAPIKey(ctx, cfg.NodeAPIKey)
	if err != nil {
		return nil, err
	}

	if err := config.UpdateNodeAPIKey(config.SystemConfigPath, renewed.NodeAPIKey, renewed.APIKeyExpiresAt); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	return &RenewKeyResult{
		NodeAPIKey:      renewed.NodeAPIKey,
		APIKeyExpiresAt: renewed.APIKeyExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func systemdUnit(agentBinPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Atellar Node Agent
After=network-online.target containerd.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, agentBinPath)
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %v failed: %s: %w", args, string(output), err)
	}
	return nil
}
