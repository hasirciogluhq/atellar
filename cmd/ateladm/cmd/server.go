package cmd

import (
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/cli/ateladm/server"
	"github.com/spf13/cobra"
)

var (
	serverDatabaseURL         string
	serverMigrationsPath      string
	serverPort                string
	serverGRPCPort            string
	serverClusterOverlayCIDR  string
	serverNodeSubnetPrefixLen int
	serverStart               bool
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Control plane API server setup",
}

var serverInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Write /etc/atellar/api.env and install atellar-api systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := server.Install(server.InstallOptions{
			DatabaseURL:         serverDatabaseURL,
			MigrationsPath:      serverMigrationsPath,
			Port:                serverPort,
			GRPCPort:            serverGRPCPort,
			ClusterOverlayCIDR:  serverClusterOverlayCIDR,
			NodeSubnetPrefixLen: serverNodeSubnetPrefixLen,
			Start:               serverStart,
		})
		if err != nil {
			return err
		}

		fmt.Printf("api server installed\n  env:  %s\n  unit: %s\n", result.EnvPath, result.UnitPath)
		if serverStart {
			fmt.Printf("  status: systemctl status atellar-api\n")
		} else {
			fmt.Printf("\nnext: sudo systemctl start atellar-api\n")
		}
		return nil
	},
}

func init() {
	serverInstallCmd.Flags().StringVar(&serverDatabaseURL, "database-url", "", "PostgreSQL connection string (required)")
	serverInstallCmd.Flags().StringVar(&serverMigrationsPath, "migrations-path", server.DefaultMigrations, "SQL migrations directory")
	serverInstallCmd.Flags().StringVar(&serverPort, "port", "8080", "HTTP listen port")
	serverInstallCmd.Flags().StringVar(&serverGRPCPort, "grpc-port", "9090", "gRPC listen port")
	serverInstallCmd.Flags().StringVar(&serverClusterOverlayCIDR, "cluster-overlay-cidr", "10.0.0.0/8", "cluster overlay CIDR")
	serverInstallCmd.Flags().IntVar(&serverNodeSubnetPrefixLen, "node-subnet-prefix-len", 24, "per-node overlay subnet prefix length")
	serverInstallCmd.Flags().BoolVar(&serverStart, "start", true, "start atellar-api after install")

	_ = serverInstallCmd.MarkFlagRequired("database-url")

	serverCmd.AddCommand(serverInstallCmd)
	rootCmd.AddCommand(serverCmd)
}
