package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/agent"
	"github.com/hasirciogluhq/atellar/pkg/config/agentconfig"
)

func (e *env) agentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Node agent"}
	path := agentconfig.Path
	run := &cobra.Command{
		Use:   "run",
		Short: "Run the node agent (started by systemd after `atellar node join`)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, err := agentconfig.Load(path)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return agent.New(path, cfg).Run(ctx)
		},
	}
	run.Flags().StringVar(&path, "agent-config", agentconfig.Path, "agent configuration file")
	cmd.AddCommand(run)
	return cmd
}
