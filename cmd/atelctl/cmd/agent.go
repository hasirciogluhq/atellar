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
	joinToken         string
	controlPlaneURL   string
	nodeName          string
	publicIP          string
	privateIP         string
	containerdSocket  string
	heartbeatInterval string

	installAutoJoin bool

	renewControlPlaneURL string
	renewAPIKey          string
	renewUpdateConfig    bool
)

func joinOptions() agent.JoinOptions {
	return agent.JoinOptions{
		JoinToken:         joinToken,
		ControlPlaneURL:   controlPlaneURL,
		NodeName:          nodeName,
		PublicIP:          publicIP,
		PrivateIP:         privateIP,
		ContainerdSocket:  containerdSocket,
		HeartbeatInterval: heartbeatInterval,
	}
}

var agentJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join this node to the control plane",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := agent.Join(context.Background(), joinOptions())
		if err != nil {
			return err
		}

		fmt.Printf("node joined\n  node_id: %s\n  config: %s\n", cfg.NodeID, config.SystemConfigPath)
		return nil
	},
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Create dirs and install atellar-agent systemd service",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !installAutoJoin {
			return nil
		}
		return agent.ValidateJoinOptions(joinOptions())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := agent.Install(context.Background(), installAutoJoin, joinOptions())
		if err != nil {
			return err
		}

		fmt.Printf("agent service installed\n  unit: %s\n", result.UnitPath)
		if result.NodeID != "" {
			fmt.Printf("  node_id: %s\n  config: %s\n", result.NodeID, config.SystemConfigPath)
		} else {
			fmt.Printf("\nnext: atelctl agent join --join-token <TOKEN> --name <NODE_NAME> --public-ip <IP> --private-ip <IP>\n")
		}
		return nil
	},
}

var agentRenewKeyCmd = &cobra.Command{
	Use:   "renew-key",
	Short: "Renew the node API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := agent.RenewKey(context.Background(), agent.RenewKeyOptions{
			ControlPlaneURL: renewControlPlaneURL,
			NodeAPIKey:      renewAPIKey,
			UpdateConfig:    renewUpdateConfig,
		})
		if err != nil {
			return err
		}

		fmt.Printf("node api key renewed\n  expires_at: %s\n  config: %s\n",
			result.APIKeyExpiresAt, config.SystemConfigPath)
		return nil
	},
}

func init() {
	agentCmd.PersistentFlags().StringVar(&joinToken, "join-token", "", "cluster join token")
	agentCmd.PersistentFlags().StringVar(&controlPlaneURL, "control-plane-url", "http://localhost:8080", "control plane URL")
	agentCmd.PersistentFlags().StringVar(&nodeName, "name", "", "node name")
	agentCmd.PersistentFlags().StringVar(&publicIP, "public-ip", "", "node public IP address")
	agentCmd.PersistentFlags().StringVar(&privateIP, "private-ip", "", "node private IP address")
	agentCmd.PersistentFlags().StringVar(&containerdSocket, "containerd-sock", "/run/containerd/containerd.sock", "containerd Unix socket path")
	agentCmd.PersistentFlags().StringVar(&heartbeatInterval, "heartbeat-interval", "5s", "agent heartbeat interval written to config")

	agentInstallCmd.Flags().BoolVar(&installAutoJoin, "auto-join", false, "run join after install")

	agentRenewKeyCmd.Flags().StringVar(&renewControlPlaneURL, "control-plane-url", "", "control plane URL")
	agentRenewKeyCmd.Flags().StringVar(&renewAPIKey, "api-key", "", "current node api key")
	agentRenewKeyCmd.Flags().BoolVar(&renewUpdateConfig, "update-config", true, "write renewed key to config")

	for _, flag := range []string{"join-token", "name", "public-ip", "private-ip"} {
		_ = agentJoinCmd.MarkPersistentFlagRequired(flag)
	}

	agentCmd.AddCommand(agentJoinCmd, agentInstallCmd, agentRenewKeyCmd)
	rootCmd.AddCommand(agentCmd)
}
