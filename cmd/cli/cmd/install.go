package cmd

import (
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/cli/install"
	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/spf13/cobra"
)

var (
	installAgentBinSource string
	installAgentBinTarget string
	installConfigPath     string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start the Atellar agent as a systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := install.Execute(install.Options{
			AgentBinSource: installAgentBinSource,
			AgentBinTarget: installAgentBinTarget,
			ConfigPath:     installConfigPath,
		})
		if err != nil {
			return err
		}

		fmt.Printf("agent installed and started\n")
		fmt.Printf("  binary: %s\n", result.AgentBinPath)
		fmt.Printf("  config: %s\n", result.ConfigPath)
		fmt.Printf("  unit:   %s\n", result.UnitPath)
		return nil
	},
}

func init() {
	installCmd.Flags().StringVar(&installAgentBinSource, "agent-bin", "", "path to atellar-agent binary (required)")
	installCmd.Flags().StringVar(&installAgentBinTarget, "target", install.DefaultAgentBinPath, "install destination for agent binary")
	installCmd.Flags().StringVar(&installConfigPath, "config", config.SystemConfigPath, "agent config path")

	_ = installCmd.MarkFlagRequired("agent-bin")

	rootCmd.AddCommand(installCmd)
}
