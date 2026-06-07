package cmd

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/cli/join"
	"github.com/hasirciogluhq/atellar/internal/pkg/agentconfig"
	"github.com/spf13/cobra"
)

var (
	joinToken             string
	joinControlPlaneURL   string
	joinNodeName          string
	joinPublicIP          string
	joinPrivateIP         string
	joinContainerdSock    string
	joinHeartbeatInterval string
	joinConfigPath        string
)

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Register this machine to the control plane and write agent config",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := join.Execute(context.Background(), join.Options{
			Token:             joinToken,
			ControlPlaneURL:   joinControlPlaneURL,
			NodeName:          joinNodeName,
			PublicIP:          joinPublicIP,
			PrivateIP:         joinPrivateIP,
			ContainerdSock:    joinContainerdSock,
			HeartbeatInterval: joinHeartbeatInterval,
			ConfigPath:        joinConfigPath,
		})
		if err != nil {
			return err
		}

		configPath := joinConfigPath
		if configPath == "" {
			configPath = agentconfig.SystemConfigPath
		}

		fmt.Printf("node joined successfully\n")
		fmt.Printf("  node_id: %s\n", cfg.NodeID)
		fmt.Printf("  config:  %s\n", configPath)
		fmt.Printf("\nnext: sudo atellar install --agent-bin <path-to-atellar-agent>\n")
		return nil
	},
}

func init() {
	joinCmd.Flags().StringVar(&joinToken, "token", "", "join token from control plane (required)")
	joinCmd.Flags().StringVar(&joinControlPlaneURL, "control-plane-url", "http://localhost:8080", "control plane base URL")
	joinCmd.Flags().StringVar(&joinNodeName, "name", "", "node name")
	joinCmd.Flags().StringVar(&joinPublicIP, "public-ip", "", "node public IP")
	joinCmd.Flags().StringVar(&joinPrivateIP, "private-ip", "", "node private IP")
	joinCmd.Flags().StringVar(&joinContainerdSock, "containerd-sock", "/run/containerd/containerd.sock", "containerd socket path")
	joinCmd.Flags().StringVar(&joinHeartbeatInterval, "heartbeat-interval", "5s", "agent heartbeat interval written to config")
	joinCmd.Flags().StringVar(&joinConfigPath, "config", agentconfig.SystemConfigPath, "agent config output path")

	_ = joinCmd.MarkFlagRequired("token")

	rootCmd.AddCommand(joinCmd)
}
