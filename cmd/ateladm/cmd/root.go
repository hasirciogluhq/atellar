package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ateladm",
	Short: "Atellar admin tool — manage servers and nodes",
	Long: `ateladm performs operator tasks such as installing the control plane,
joining nodes, and installing the node agent service.

Examples:
  sudo ateladm server install --database-url postgresql://user:pass@localhost:5432/atellar_cp?sslmode=disable
  sudo ateladm node install --auto-join --join-token <TOKEN> --name node-1 --public-ip <IP> --private-ip <IP>`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
