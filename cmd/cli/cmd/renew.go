package cmd

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/cli/renew"
	"github.com/hasirciogluhq/atellar/internal/pkg/agentconfig"
	"github.com/spf13/cobra"
)

var (
	renewControlPlaneURL string
	renewNodeAPIKey      string
	renewConfigPath      string
	renewUpdateConfig    bool
)

var renewCmd = &cobra.Command{
	Use:   "renew-api-key",
	Short: "Renew the node API key using current credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := renew.Execute(context.Background(), renew.Options{
			ControlPlaneURL: renewControlPlaneURL,
			NodeAPIKey:      renewNodeAPIKey,
			ConfigPath:      renewConfigPath,
			UpdateConfig:    renewUpdateConfig,
		})
		if err != nil {
			return err
		}

		fmt.Printf("node api key renewed\n")
		fmt.Printf("  node_api_key:       %s\n", result.NodeAPIKey)
		fmt.Printf("  api_key_expires_at: %s\n", result.APIKeyExpiresAt)
		if renewUpdateConfig || renewConfigPath != "" {
			fmt.Printf("  config:             %s\n", result.ConfigPath)
		}
		return nil
	},
}

func init() {
	renewCmd.Flags().StringVar(&renewControlPlaneURL, "control-plane-url", "", "control plane base URL")
	renewCmd.Flags().StringVar(&renewNodeAPIKey, "api-key", "", "current node api key")
	renewCmd.Flags().StringVar(&renewConfigPath, "config", agentconfig.SystemConfigPath, "agent config path")
	renewCmd.Flags().BoolVar(&renewUpdateConfig, "update-config", true, "write renewed api key to config file")

	rootCmd.AddCommand(renewCmd)
}
