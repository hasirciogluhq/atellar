package cli

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/spf13/cobra"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/host"
	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/server"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
	"github.com/hasirciogluhq/atellar/pkg/config/serverconfig"
	"github.com/hasirciogluhq/atellar/pkg/storage/postgres"
	"github.com/hasirciogluhq/atellar/pkg/version"
)

func (e *env) serverCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "server", Short: "API server setup"}

	cfg := serverconfig.Default()
	var (
		open  bool
		start bool
	)
	install := &cobra.Command{
		Use:   "install",
		Short: "Write " + serverconfig.EnvPath + " and install the " + host.APIUnit + " service",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			generated := false
			switch {
			case open && cfg.AdminToken != "":
				return fmt.Errorf("--open and --admin-token are mutually exclusive")
			case !open && cfg.AdminToken == "":
				tok, err := token.Generate()
				if err != nil {
					return err
				}
				cfg.AdminToken, generated = tok, true
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := e.host.RequireFile(host.Binary, "install the release first"); err != nil {
				return err
			}
			if err := e.host.RequireFile(cfg.MigrationsPath, "install the release first or pass --migrations-path"); err != nil {
				return err
			}
			if err := e.host.WriteFile(serverconfig.EnvPath, []byte(cfg.Env()), 0o600); err != nil {
				return err
			}
			if err := e.host.InstallUnit(host.APIUnit, host.APIUnitFile(serverconfig.EnvPath), start); err != nil {
				return err
			}

			fmt.Fprintf(e.out, "API server installed\n  config: %s\n  unit:   %s\n", serverconfig.EnvPath, host.APIUnit)
			if open {
				fmt.Fprintf(e.out, "\nWARNING: no admin token. Anyone who can reach ports %d and %d is cluster admin.\n", cfg.HTTPPort, cfg.GRPCPort)
				return nil
			}
			if generated {
				fmt.Fprintf(e.out, "\nAdmin token (stored only in %s):\n  %s\n", serverconfig.EnvPath, cfg.AdminToken)
			}
			fmt.Fprintf(e.out, "\nPoint the CLI at it:\n  atellar config set-cluster default --address <THIS HOST> --http-port %d --grpc-port %d --token <ADMIN TOKEN>\n  atellar config set-context default --cluster default\n  atellar config use-context default\n",
				cfg.HTTPPort, cfg.GRPCPort)
			return nil
		},
	}
	f := install.Flags()
	f.StringVar(&cfg.DatabaseURL, "database-url", "", "PostgreSQL connection string")
	f.StringVar(&cfg.MigrationsPath, "migrations-path", cfg.MigrationsPath, "SQL migrations directory")
	f.StringVar(&cfg.BindAddress, "bind-address", "", "listen address; empty listens on every interface")
	f.IntVar(&cfg.HTTPPort, "http-port", cfg.HTTPPort, "HTTP port")
	f.IntVar(&cfg.GRPCPort, "grpc-port", cfg.GRPCPort, "gRPC port")
	f.StringVar(&cfg.ClusterCIDR, "cluster-cidr", cfg.ClusterCIDR, "IPv4 range split into node subnets")
	f.IntVar(&cfg.NodeSubnetPrefixLen, "node-subnet-prefix", cfg.NodeSubnetPrefixLen, "prefix length of each node subnet")
	f.DurationVar(&cfg.NodeStaleAfter, "node-stale-after", cfg.NodeStaleAfter, "mark a node not_ready after this long without a heartbeat")
	f.StringVar(&cfg.AdminToken, "admin-token", "", "admin token; generated when empty")
	f.BoolVar(&open, "open", false, "run without an admin token (trusted networks only)")
	f.BoolVar(&start, "start", true, "start the service after installing")
	_ = install.MarkFlagRequired("database-url")

	cmd.AddCommand(install, serverRunCmd())
	return cmd
}

// serverRunCmd is what the systemd unit starts.
func serverRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the API server (configured by environment, see " + serverconfig.EnvPath + ")",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, err := serverconfig.FromOSEnv()
			if err != nil {
				return fmt.Errorf("configuration: %w", err)
			}
			ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := postgres.Migrate(ctx, cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("database: %w", err)
			}
			defer pool.Close()
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("database: %w", err)
			}
			log.Printf("atellar server %s starting", version.Version)
			return server.Run(ctx, cfg, postgres.New(pool), server.Listeners{})
		},
	}
}
