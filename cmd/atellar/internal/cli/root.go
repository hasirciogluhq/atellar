// Package cli implements the atellar command line. One binary covers every
// role: operating a cluster, installing and running the API server, and
// joining and running a node agent.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/host"
	"github.com/hasirciogluhq/atellar/pkg/config/ctlconfig"
	"github.com/hasirciogluhq/atellar/pkg/transport"
	"github.com/hasirciogluhq/atellar/pkg/version"
)

// env is shared by every command of one invocation.
type env struct {
	out        io.Writer
	host       host.Host
	configPath string
	context    string
	output     string
}

// NewRoot builds the command tree writing to out; h decides where machine
// level files go.
func NewRoot(out io.Writer, h host.Host) *cobra.Command {
	e := &env{out: out, host: h}
	root := &cobra.Command{
		Use:   "atellar",
		Short: "Run and operate an Atellar cluster",
		Long: `atellar installs, runs and operates an Atellar cluster.

Set up the control plane and nodes (as root):

  atellar server install --database-url postgres://...
  atellar node install
  atellar node join --join-token <TOKEN> --name node-1 --private-ip 10.0.0.21 --control-plane-address 10.0.0.10

Point the CLI at the cluster once:

  atellar config set-cluster prod --address 10.0.0.10 --token <admin token>
  atellar config set-context prod --cluster prod
  atellar config use-context prod

Then operate it:

  atellar nodes list
  atellar join-tokens create
  atellar containers create --image docker.io/library/nginx:1.27

systemd runs "atellar server run" and "atellar agent run"; they are not
meant to be started by hand.`,
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.PersistentFlags().StringVar(&e.configPath, "config", ctlconfig.DefaultPath(), "client configuration file")
	root.PersistentFlags().StringVar(&e.context, "context", "", "context to use instead of the current one")
	root.PersistentFlags().StringVarP(&e.output, "output", "o", "table", "output format: table or json")
	root.AddCommand(
		e.configCmd(), e.nodesCmd(), e.containersCmd(), e.joinTokensCmd(),
		e.serverCmd(), e.nodeCmd(), e.agentCmd(), e.versionCmd(),
	)
	return root
}

func (e *env) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintln(e.out, version.Version)
			return err
		},
	}
}

// Execute runs atellar with the process arguments.
func Execute() {
	if err := NewRoot(os.Stdout, host.Host{}).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// cluster resolves --context or the current context.
func (e *env) cluster() (ctlconfig.Cluster, error) {
	cfg, err := ctlconfig.Load(e.configPath)
	if err != nil {
		return ctlconfig.Cluster{}, err
	}
	if e.context != "" {
		if err := cfg.UseContext(e.context); err != nil {
			return ctlconfig.Cluster{}, err
		}
	}
	return cfg.Current()
}

func (e *env) connect() (transport.Client, error) {
	cluster, err := e.cluster()
	if err != nil {
		return nil, err
	}
	return cluster.Client()
}

// print writes v as JSON, or as a table built from header and rows.
func (e *env) print(v any, header []string, rows [][]string) error {
	switch e.output {
	case "json":
		enc := json.NewEncoder(e.out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case "table", "":
		w := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, strings.Join(header, "\t"))
		for _, row := range rows {
			fmt.Fprintln(w, strings.Join(row, "\t"))
		}
		return w.Flush()
	}
	return fmt.Errorf("unknown output format %q", e.output)
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
