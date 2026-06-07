package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "atelctl",
	Short: "Atellar control tool — manage cluster and node agents",
	Long: `atelctl talks to the control plane (cluster) and configures local node agents.

Examples:
  atelctl agent init
  atelctl agent join --token <TOKEN> --name node-1
  atelctl agent install --agent-bin ./atellar-agent
  atelctl cluster nodes list`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
