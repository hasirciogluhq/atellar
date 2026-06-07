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
	agentInitToken             string
	agentInitControlPlaneURL   string
	agentInitNodeName          string
	agentInitPublicIP          string
	agentInitPrivateIP         string
	agentInitContainerdSock    string
	agentInitHeartbeatInterval string
	agentInitConfigPath        string

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
	Short: "Register this node and write agent config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := agent.Init(context.Background(), agent.InitOptions{
			Token:             agentInitToken,
			ControlPlaneURL:   agentInitControlPlaneURL,
			NodeName:          agentInitNodeName,
			PublicIP:          agentInitPublicIP,
			PrivateIP:         agentInitPrivateIP,
			ContainerdSock:    agentInitContainerdSock,
			HeartbeatInterval: agentInitHeartbeatInterval,
			ConfigPath:        agentInitConfigPath,
		})
		if err != nil {
			return err
		}

		configPath := agentInitConfigPath
		if configPath == "" {
			configPath = config.SystemConfigPath
		}

		fmt.Printf("agent initialized\n")
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
	agentInitCmd.Flags().StringVar(&agentInitToken, "token", "", "join token from control plane (required)")
	agentInitCmd.Flags().StringVar(&agentInitControlPlaneURL, "control-plane-url", "http://localhost:8080", "control plane base URL")
	agentInitCmd.Flags().StringVar(&agentInitNodeName, "name", "", "node name")
	agentInitCmd.Flags().StringVar(&agentInitPublicIP, "public-ip", "", "node public IP")
	agentInitCmd.Flags().StringVar(&agentInitPrivateIP, "private-ip", "", "node private IP")
	agentInitCmd.Flags().StringVar(&agentInitContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "containerd socket path")
	agentInitCmd.Flags().StringVar(&agentInitHeartbeatInterval, "heartbeat-interval", "5s", "agent heartbeat interval")
	agentInitCmd.Flags().StringVar(&agentInitConfigPath, "config", config.SystemConfigPath, "agent config output path")
	_ = agentInitCmd.MarkFlagRequired("token")

	agentInstallCmd.Flags().StringVar(&agentInstallBinSource, "agent-bin", "", "path to atellar-agent binary (required)")
	agentInstallCmd.Flags().StringVar(&agentInstallBinTarget, "target", agent.DefaultAgentBinPath, "install destination")
	agentInstallCmd.Flags().StringVar(&agentInstallConfigPath, "config", config.SystemConfigPath, "agent config path")
	_ = agentInstallCmd.MarkFlagRequired("agent-bin")

	agentRenewKeyCmd.Flags().StringVar(&agentRenewControlPlaneURL, "control-plane-url", "", "control plane base URL")
	agentRenewKeyCmd.Flags().StringVar(&agentRenewAPIKey, "api-key", "", "current node api key")
	agentRenewKeyCmd.Flags().StringVar(&agentRenewConfigPath, "config", config.SystemConfigPath, "agent config path")
	agentRenewKeyCmd.Flags().BoolVar(&agentRenewUpdateConfig, "update-config", true, "write renewed key to config")

	agentCmd.AddCommand(agentInitCmd, agentInstallCmd, agentRenewKeyCmd)
	rootCmd.AddCommand(agentCmd)
}
