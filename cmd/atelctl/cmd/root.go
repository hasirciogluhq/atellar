package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "atelctl",
	Short: "Atellar client — inspect and operate clusters",
	Long: `atelctl talks to Atellar control planes as an end-user client.

Examples:
  atelctl config set-cluster local --control-plane-address 127.0.0.1 --http-port 8080 --grpc-port 9090
  atelctl config set-context local --cluster local
  atelctl config use-context local
  atelctl cluster nodes list`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
