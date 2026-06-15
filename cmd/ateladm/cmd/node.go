package cmd

import (
	"context"
	"fmt"

	agentconfig "github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/internal/cli/ateladm/agent"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/spf13/cobra"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Node setup and maintenance",
}

var (
	joinToken           string
	controlPlaneAddress string
	httpPort            int
	grpcPort            int
	nodeName            string
	publicIP            string
	privateIP           string
	containerdSocket    string
	heartbeatInterval   string

	installAutoJoin bool
)

func joinOptions() agent.JoinOptions {
	return agent.JoinOptions{
		JoinToken: joinToken,
		ControlPlane: client.ControlPlane{
			Address:  controlPlaneAddress,
			HTTPPort: httpPort,
			GRPCPort: grpcPort,
		},
		NodeName:          nodeName,
		PublicIP:          publicIP,
		PrivateIP:         privateIP,
		ContainerdSocket:  containerdSocket,
		HeartbeatInterval: heartbeatInterval,
	}
}

func joinOptionsFromFlagsOrConfig() (agent.JoinOptions, error) {
	cp, err := resolveControlPlaneFromFlagsOrConfig(controlPlaneAddress, httpPort, grpcPort)
	if err != nil {
		return agent.JoinOptions{}, err
	}

	opts := joinOptions()
	opts.ControlPlane = cp
	return opts, nil
}

var nodeJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join this node to the control plane",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := joinOptionsFromFlagsOrConfig()
		if err != nil {
			return err
		}

		cfg, err := agent.Join(context.Background(), opts)
		if err != nil {
			return err
		}

		fmt.Printf("node joined\n  node_id: %s\n  config: %s\n", cfg.NodeID, agentconfig.SystemConfigPath)
		return nil
	},
}

var nodeInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Create dirs and install atelagent systemd service",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !installAutoJoin {
			return nil
		}
		opts, err := joinOptionsFromFlagsOrConfig()
		if err != nil {
			return err
		}
		return agent.ValidateJoinOptions(opts)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := joinOptions()
		if installAutoJoin {
			resolved, err := joinOptionsFromFlagsOrConfig()
			if err != nil {
				return err
			}
			opts = resolved
		}

		result, err := agent.Install(context.Background(), installAutoJoin, opts)
		if err != nil {
			return err
		}

		fmt.Printf("node agent service installed\n  unit: %s\n", result.UnitPath)
		if result.NodeID != "" {
			fmt.Printf("  node_id: %s\n  config: %s\n", result.NodeID, agentconfig.SystemConfigPath)
		} else {
			fmt.Printf("\nnext: ateladm node join --join-token <TOKEN> --name <NODE> --public-ip <IP> --private-ip <IP>\n")
		}
		return nil
	},
}

var nodeRenewKeyCmd = &cobra.Command{
	Use:   "renew-key",
	Short: "Renew the node API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := agent.RenewKey(context.Background())
		if err != nil {
			return err
		}

		fmt.Printf("node api key renewed\n  expires_at: %s\n  config: %s\n",
			result.APIKeyExpiresAt, agentconfig.SystemConfigPath)
		return nil
	},
}

func init() {
	nodeCmd.PersistentFlags().StringVar(&joinToken, "join-token", "", "cluster join token")
	nodeCmd.PersistentFlags().StringVar(&controlPlaneAddress, "control-plane-address", "", "control plane host or IP")
	nodeCmd.PersistentFlags().IntVar(&httpPort, "http-port", 0, "control plane HTTP port")
	nodeCmd.PersistentFlags().IntVar(&grpcPort, "grpc-port", 0, "control plane gRPC port")
	nodeCmd.PersistentFlags().StringVar(&nodeName, "name", "", "node name")
	nodeCmd.PersistentFlags().StringVar(&publicIP, "public-ip", "", "node public IP address")
	nodeCmd.PersistentFlags().StringVar(&privateIP, "private-ip", "", "node private IP address")
	nodeCmd.PersistentFlags().StringVar(&containerdSocket, "containerd-sock", "/run/containerd/containerd.sock", "containerd Unix socket path")
	nodeCmd.PersistentFlags().StringVar(&heartbeatInterval, "heartbeat-interval", "5s", "agent heartbeat interval written to config")

	nodeInstallCmd.Flags().BoolVar(&installAutoJoin, "auto-join", false, "run join after install")

	for _, flag := range []string{"join-token", "name", "public-ip", "private-ip"} {
		_ = nodeJoinCmd.MarkPersistentFlagRequired(flag)
	}

	nodeCmd.AddCommand(nodeJoinCmd, nodeInstallCmd, nodeRenewKeyCmd)
	rootCmd.AddCommand(nodeCmd)
}
