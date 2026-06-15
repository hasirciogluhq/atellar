package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	atelconfig "github.com/hasirciogluhq/atellar/internal/cli/atelctl/config"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/spf13/cobra"
)

var (
	configPath string

	configContextCluster string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage atelctl clusters and contexts",
}

var configSetClusterCmd = &cobra.Command{
	Use:   "set-cluster NAME",
	Short: "Set a control plane cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := atelconfig.Load(configPath)
		if err != nil {
			return err
		}

		cluster := atelconfig.Cluster{
			Address:  clusterControlPlaneAddress,
			HTTPPort: clusterHTTPPort,
			GRPCPort: clusterGRPCPort,
		}
		if err := cfg.SetCluster(args[0], cluster); err != nil {
			return err
		}
		if err := atelconfig.Save(configPath, cfg); err != nil {
			return err
		}

		fmt.Printf("cluster %q set\n", args[0])
		return nil
	},
}

var configSetContextCmd = &cobra.Command{
	Use:   "set-context NAME --cluster CLUSTER",
	Short: "Set a context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := atelconfig.Load(configPath)
		if err != nil {
			return err
		}

		if err := cfg.SetContext(args[0], configContextCluster); err != nil {
			return err
		}
		if err := atelconfig.Save(configPath, cfg); err != nil {
			return err
		}

		fmt.Printf("context %q set\n", args[0])
		return nil
	},
}

var configUseContextCmd = &cobra.Command{
	Use:   "use-context NAME",
	Short: "Set the current context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := atelconfig.Load(configPath)
		if err != nil {
			return err
		}

		if err := cfg.UseContext(args[0]); err != nil {
			return err
		}
		if err := atelconfig.Save(configPath, cfg); err != nil {
			return err
		}

		fmt.Printf("current context set to %q\n", args[0])
		return nil
	},
}

var configCurrentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Print the current context",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := atelconfig.Load(configPath)
		if err != nil {
			return err
		}
		if cfg.CurrentContext == "" {
			return fmt.Errorf("current context is not set")
		}
		fmt.Println(cfg.CurrentContext)
		return nil
	},
}

var configGetContextsCmd = &cobra.Command{
	Use:   "get-contexts",
	Short: "List contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := atelconfig.Load(configPath)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CURRENT\tNAME\tCLUSTER\tADDRESS\tHTTP\tGRPC")
		for _, name := range cfg.ContextNames() {
			ctx := cfg.Contexts[name]
			cluster := cfg.Clusters[ctx.Cluster]
			current := ""
			if name == cfg.CurrentContext {
				current = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\n",
				current, name, ctx.Cluster, cluster.Address, cluster.HTTPPort, cluster.GRPCPort)
		}
		return w.Flush()
	},
}

func resolveControlPlaneFromFlagsOrConfig(address string, httpPort, grpcPort int) (client.ControlPlane, error) {
	if address != "" || httpPort != 0 || grpcPort != 0 {
		cp := client.ControlPlane{Address: address, HTTPPort: httpPort, GRPCPort: grpcPort}
		if err := cp.Validate(); err != nil {
			return client.ControlPlane{}, err
		}
		return cp, nil
	}

	cfg, err := atelconfig.Load(configPath)
	if err != nil {
		return client.ControlPlane{}, err
	}

	cluster, err := cfg.ResolveCurrentCluster()
	if err != nil {
		return client.ControlPlane{}, fmt.Errorf("%w; run `atelctl config set-cluster`, `atelctl config set-context`, and `atelctl config use-context`, or pass endpoint flags", err)
	}
	return client.ControlPlane{
		Address:  cluster.Address,
		HTTPPort: cluster.HTTPPort,
		GRPCPort: cluster.GRPCPort,
	}, nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", atelconfig.DefaultPath(), "atelctl config file")

	configSetClusterCmd.Flags().StringVar(&clusterControlPlaneAddress, "control-plane-address", "", "control plane host or IP")
	configSetClusterCmd.Flags().IntVar(&clusterHTTPPort, "http-port", 0, "control plane HTTP port")
	configSetClusterCmd.Flags().IntVar(&clusterGRPCPort, "grpc-port", 0, "control plane gRPC port")

	for _, flag := range []string{"control-plane-address", "http-port", "grpc-port"} {
		_ = configSetClusterCmd.MarkFlagRequired(flag)
	}

	configSetContextCmd.Flags().StringVar(&configContextCluster, "cluster", "", "cluster name")
	_ = configSetContextCmd.MarkFlagRequired("cluster")

	configCmd.AddCommand(
		configSetClusterCmd,
		configSetContextCmd,
		configUseContextCmd,
		configCurrentContextCmd,
		configGetContextsCmd,
	)
	rootCmd.AddCommand(configCmd)
}
