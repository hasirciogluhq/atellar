package cmd

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/internal/atelctl/agent"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Node agent setup and maintenance",
}

var (
	agentInitContainerdSock string
	agentInitConfigDir      string
	agentInitLogDir         string

	agentJoinToken             string
	agentJoinControlPlaneURL   string
	agentJoinNodeName          string
	agentJoinPublicIP          string
	agentJoinPrivateIP         string
	agentJoinContainerdSock    string
	agentJoinHeartbeatInterval string
	agentJoinConfigPath        string

	agentInstallBinSource string
	agentInstallBinTarget string
	agentInstallConfigPath string

	agentRenewControlPlaneURL string
	agentRenewAPIKey          string
	agentRenewConfigPath      string
	agentRenewUpdateConfig    bool
)

var agentInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Prepare this node for the cluster (dirs, prerequisites)",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := agent.Init(agent.InitOptions{
			ContainerdSock: agentInitContainerdSock,
			ConfigDir:      agentInitConfigDir,
			LogDir:         agentInitLogDir,
		})
		if err != nil {
			return err
		}

		fmt.Printf("node prepared\n")
		fmt.Printf("  config_dir:      %s\n", result.ConfigDir)
		fmt.Printf("  log_dir:         %s\n", result.LogDir)
		fmt.Printf("  containerd_sock: %s\n", result.ContainerdSock)
		fmt.Printf("\nnext: atelctl agent join --token <JOIN_TOKEN> --name <NODE_NAME>\n")
		return nil
	},
}

var agentJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join this node to the control plane",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := agent.Join(context.Background(), agent.JoinOptions{
			Token:             agentJoinToken,
			ControlPlaneURL:   agentJoinControlPlaneURL,
			NodeName:          agentJoinNodeName,
			PublicIP:          agentJoinPublicIP,
			PrivateIP:         agentJoinPrivateIP,
			ContainerdSock:    agentJoinContainerdSock,
			HeartbeatInterval: agentJoinHeartbeatInterval,
			ConfigPath:        agentJoinConfigPath,
		})
		if err != nil {
			return err
		}

		configPath := agentJoinConfigPath
		if configPath == "" {
			configPath = config.SystemConfigPath
		}

		fmt.Printf("node joined\n")
		fmt.Printf("  node_id: %s\n", cfg.NodeID)
		fmt.Printf("  config:  %s\n", configPath)
		fmt.Printf("\nnext: sudo atelctl agent install --agent-bin <path-to-atellar-agent>\n")
		return nil
	},
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start the agent as a systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := agent.Install(agent.InstallOptions{
			AgentBinSource: agentInstallBinSource,
			AgentBinTarget: agentInstallBinTarget,
			ConfigPath:     agentInstallConfigPath,
		})
		if err != nil {
			return err
		}

		fmt.Printf("agent installed and started\n")
		fmt.Printf("  binary: %s\n", result.AgentBinPath)
		fmt.Printf("  config: %s\n", result.ConfigPath)
		fmt.Printf("  unit:   %s\n", result.UnitPath)
		return nil
	},
}

var agentRenewKeyCmd = &cobra.Command{
	Use:   "renew-key",
	Short: "Renew the node API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := agent.RenewKey(context.Background(), agent.RenewKeyOptions{
			ControlPlaneURL: agentRenewControlPlaneURL,
			NodeAPIKey:      agentRenewAPIKey,
			ConfigPath:      agentRenewConfigPath,
			UpdateConfig:    agentRenewUpdateConfig,
		})
		if err != nil {
			return err
		}

		fmt.Printf("node api key renewed\n")
		fmt.Printf("  node_api_key:       %s\n", result.NodeAPIKey)
		fmt.Printf("  api_key_expires_at: %s\n", result.APIKeyExpiresAt)
		if agentRenewUpdateConfig || agentRenewConfigPath != "" {
			fmt.Printf("  config:             %s\n", result.ConfigPath)
		}
		return nil
	},
}

func init() {
	agentInitCmd.Flags().StringVar(&agentInitContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "containerd socket to verify")
	agentInitCmd.Flags().StringVar(&agentInitConfigDir, "config-dir", config.SystemConfigDir, "agent config directory")
	agentInitCmd.Flags().StringVar(&agentInitLogDir, "log-dir", agent.LogDir, "agent log directory")

	agentJoinCmd.Flags().StringVar(&agentJoinToken, "token", "", "join token from control plane (required)")
	agentJoinCmd.Flags().StringVar(&agentJoinControlPlaneURL, "control-plane-url", "http://localhost:8080", "control plane base URL")
	agentJoinCmd.Flags().StringVar(&agentJoinNodeName, "name", "", "node name")
	agentJoinCmd.Flags().StringVar(&agentJoinPublicIP, "public-ip", "", "node public IP")
	agentJoinCmd.Flags().StringVar(&agentJoinPrivateIP, "private-ip", "", "node private IP")
	agentJoinCmd.Flags().StringVar(&agentJoinContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "containerd socket path")
	agentJoinCmd.Flags().StringVar(&agentJoinHeartbeatInterval, "heartbeat-interval", "5s", "agent heartbeat interval")
	agentJoinCmd.Flags().StringVar(&agentJoinConfigPath, "config", config.SystemConfigPath, "agent config output path")
	_ = agentJoinCmd.MarkFlagRequired("token")

	agentInstallCmd.Flags().StringVar(&agentInstallBinSource, "agent-bin", "", "path to atellar-agent binary (required)")
	agentInstallCmd.Flags().StringVar(&agentInstallBinTarget, "target", agent.DefaultAgentBinPath, "install destination")
	agentInstallCmd.Flags().StringVar(&agentInstallConfigPath, "config", config.SystemConfigPath, "agent config path")
	_ = agentInstallCmd.MarkFlagRequired("agent-bin")

	agentRenewKeyCmd.Flags().StringVar(&agentRenewControlPlaneURL, "control-plane-url", "", "control plane base URL")
	agentRenewKeyCmd.Flags().StringVar(&agentRenewAPIKey, "api-key", "", "current node api key")
	agentRenewKeyCmd.Flags().StringVar(&agentRenewConfigPath, "config", config.SystemConfigPath, "agent config path")
	agentRenewKeyCmd.Flags().BoolVar(&agentRenewUpdateConfig, "update-config", true, "write renewed key to config")

	agentCmd.AddCommand(agentInitCmd, agentJoinCmd, agentInstallCmd, agentRenewKeyCmd)
	rootCmd.AddCommand(agentCmd)
}
