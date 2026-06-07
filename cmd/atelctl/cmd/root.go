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
  sudo atelctl agent install --auto-join --join-token <TOKEN> --name node-1
  atelctl agent join --join-token <TOKEN> --name node-1
  atelctl cluster nodes list`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
