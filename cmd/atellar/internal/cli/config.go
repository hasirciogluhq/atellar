package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hasirciogluhq/atellar/pkg/config/ctlconfig"
	"github.com/hasirciogluhq/atellar/pkg/pki"
)

func (e *env) configCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage clusters and contexts"}

	var (
		cluster ctlconfig.Cluster
		ca      caSource
	)
	setCluster := &cobra.Command{
		Use:   "set-cluster NAME",
		Short: "Add or replace a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			caPEM, err := ca.resolve(c.Context(), cluster.Address, cluster.HTTPPort)
			if err != nil {
				return err
			}
			cluster.CACert = string(caPEM)
			return e.updateConfig(func(cfg *ctlconfig.Config) error { return cfg.SetCluster(args[0], cluster) },
				"cluster %q set", args[0])
		},
	}
	f := setCluster.Flags()
	f.StringVar(&cluster.Address, "address", "", "API server host or IP")
	f.IntVar(&cluster.HTTPPort, "http-port", 8080, "API server HTTP port")
	f.IntVar(&cluster.GRPCPort, "grpc-port", 9090, "API server gRPC port")
	f.StringVar(&cluster.Transport, "transport", "grpc", "transport: grpc or http")
	f.StringVar(&cluster.Token, "token", "", "API token")
	f.StringVar(&ca.hash, "ca-cert-hash", "", "pin of the cluster CA (sha256:...), printed by `atellar server install`; the CA is fetched and checked against it")
	f.StringVar(&ca.file, "ca-cert", "", "cluster CA certificate file (the server's "+pki.DefaultDir+"/"+pki.CACertFile+")")
	_ = setCluster.MarkFlagRequired("address")

	var contextCluster string
	setContext := &cobra.Command{
		Use:   "set-context NAME",
		Short: "Add or replace a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return e.updateConfig(func(cfg *ctlconfig.Config) error { return cfg.SetContext(args[0], contextCluster) },
				"context %q set", args[0])
		},
	}
	setContext.Flags().StringVar(&contextCluster, "cluster", "", "cluster the context points at")
	_ = setContext.MarkFlagRequired("cluster")

	useContext := &cobra.Command{
		Use:   "use-context NAME",
		Short: "Select the current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return e.updateConfig(func(cfg *ctlconfig.Config) error { return cfg.UseContext(args[0]) },
				"switched to context %q", args[0])
		},
	}

	current := &cobra.Command{
		Use:   "current-context",
		Short: "Print the current context",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := ctlconfig.Load(e.configPath)
			if err != nil {
				return err
			}
			if cfg.CurrentContext == "" {
				return fmt.Errorf("no current context")
			}
			_, err = fmt.Fprintln(e.out, cfg.CurrentContext)
			return err
		},
	}

	list := &cobra.Command{
		Use:   "get-contexts",
		Short: "List contexts",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cfg, err := ctlconfig.Load(e.configPath)
			if err != nil {
				return err
			}
			var rows [][]string
			for _, name := range cfg.ContextNames() {
				c := cfg.Clusters[cfg.Contexts[name].Cluster]
				mark := ""
				if name == cfg.CurrentContext {
					mark = "*"
				}
				ep, _ := endpoint(c)
				rows = append(rows, []string{mark, name, cfg.Contexts[name].Cluster, ep})
			}
			return e.print(cfg.Contexts, []string{"CURRENT", "NAME", "CLUSTER", "ENDPOINT"}, rows)
		},
	}

	cmd.AddCommand(setCluster, setContext, useContext, current, list)
	return cmd
}

func endpoint(c ctlconfig.Cluster) (string, error) {
	kind := or(c.Transport, "grpc")
	port := c.GRPCPort
	if kind == "http" {
		port = c.HTTPPort
	}
	return kind + "://" + c.Address + ":" + strconv.Itoa(port), nil
}

func (e *env) updateConfig(change func(*ctlconfig.Config) error, format string, args ...any) error {
	cfg, err := ctlconfig.Load(e.configPath)
	if err != nil {
		return err
	}
	if err := change(cfg); err != nil {
		return err
	}
	if err := ctlconfig.Save(e.configPath, cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.out, format+"\n", args...)
	return err
}
