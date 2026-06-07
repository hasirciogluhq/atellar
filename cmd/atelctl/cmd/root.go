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
  sudo atelctl server install --database-url postgresql://user:pass@localhost:5432/atellar_cp?sslmode=disable
  sudo atelctl agent install --auto-join --join-token <TOKEN> --name node-1 --public-ip <IP> --private-ip <IP> --control-plane-address <HOST> --http-port 8080 --grpc-port 9090
  atelctl cluster nodes list`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
