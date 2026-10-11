package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/host"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/config/agentconfig"
	"github.com/hasirciogluhq/atellar/pkg/pki"
	"github.com/hasirciogluhq/atellar/pkg/version"
)

func (e *env) nodeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "node", Short: "Node agent setup"}
	cmd.AddCommand(e.nodeInstallCmd(), e.nodeJoinCmd(), e.nodeRenewKeyCmd(), e.nodeTrustCACmd())
	return cmd
}

func (e *env) nodeInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and enable the node agent service",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if err := e.host.RequireFile(host.Binary, "install the release first"); err != nil {
				return err
			}
			if err := os.MkdirAll(e.host.Path(host.LogDir), 0o755); err != nil {
				return err
			}
			// The agent starts once `node join` wrote its configuration.
			if err := e.host.InstallUnit(host.AgentUnit, host.AgentUnitFile(), false); err != nil {
				return err
			}
			fmt.Fprintf(e.out, "node agent service installed (%s)\nnext: sudo atellar node join --join-token <TOKEN> --ca-cert-hash <HASH> --name <NAME> --private-ip <IP> --control-plane-address <API HOST>\n", host.AgentUnit)
			return nil
		},
	}
}

type joinOptions struct {
	token          string
	ca             caSource
	controlPlane   client.ControlPlane
	transport      string
	name           string
	publicIP       string
	privateIP      string
	containerdSock string
	heartbeat      time.Duration
	skipSocket     bool
}

func (e *env) nodeJoinCmd() *cobra.Command {
	var o joinOptions
	join := &cobra.Command{
		Use:   "join",
		Short: "Register this machine with the API server and write " + agentconfig.Path,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return e.join(c.Context(), o)
		},
	}
	f := join.Flags()
	f.StringVar(&o.token, "join-token", "", "join token from `atellar join-tokens create`")
	f.StringVar(&o.ca.hash, "ca-cert-hash", "", "pin of the cluster CA (sha256:...), printed by `atellar join-tokens create`")
	f.StringVar(&o.ca.file, "ca-cert", "", "cluster CA certificate file, instead of fetching it")
	f.StringVar(&o.controlPlane.Address, "control-plane-address", "", "API server host or IP")
	f.IntVar(&o.controlPlane.HTTPPort, "http-port", 8080, "API server HTTP port")
	f.IntVar(&o.controlPlane.GRPCPort, "grpc-port", 9090, "API server gRPC port")
	f.StringVar(&o.transport, "transport", "grpc", "transport the agent uses: grpc or http")
	f.StringVar(&o.name, "name", "", "node name, unique in the cluster")
	f.StringVar(&o.publicIP, "public-ip", "", "public address (informational)")
	f.StringVar(&o.privateIP, "private-ip", "", "address other nodes reach this node on; carries VXLAN traffic")
	f.StringVar(&o.containerdSock, "containerd-sock", agentconfig.DefaultContainerdSock, "containerd socket")
	f.DurationVar(&o.heartbeat, "heartbeat-interval", agentconfig.DefaultHeartbeatInterval, "agent heartbeat interval")
	f.BoolVar(&o.skipSocket, "skip-containerd-check", false, "do not require the containerd socket to exist")
	_ = f.MarkHidden("skip-containerd-check")
	for _, name := range []string{"join-token", "control-plane-address", "name", "private-ip"} {
		_ = join.MarkFlagRequired(name)
	}
	return join
}

func (e *env) join(ctx context.Context, o joinOptions) error {
	if !o.skipSocket {
		if _, err := os.Stat(o.containerdSock); err != nil {
			return fmt.Errorf("containerd socket %s not found; install and start containerd first", o.containerdSock)
		}
	}
	caPEM, err := o.ca.resolve(ctx, o.controlPlane.Address, o.controlPlane.HTTPPort)
	if err != nil {
		return err
	}
	tlsConfig, err := pki.ClientConfig(caPEM)
	if err != nil {
		return err
	}
	c, err := client.New(client.Config{ControlPlane: o.controlPlane, Transport: o.transport, TLS: tlsConfig})
	if err != nil {
		return err
	}
	defer c.Close()

	resp, err := c.Nodes().Register(ctx, v1.RegisterNodeRequest{
		JoinToken:      o.token,
		Name:           o.name,
		PublicIP:       o.publicIP,
		PrivateIP:      o.privateIP,
		AgentVersion:   version.Version,
		ContainerdSock: o.containerdSock,
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	cfg := &agentconfig.Config{
		ControlPlane:      o.controlPlane,
		Transport:         o.transport,
		CACert:            string(caPEM),
		NodeID:            resp.Node.ID,
		NodeName:          resp.Node.Name,
		NodeAPIKey:        resp.NodeAPIKey,
		APIKeyExpiresAt:   resp.APIKeyExpiresAt,
		ContainerdSock:    o.containerdSock,
		HeartbeatInterval: agentconfig.Duration(o.heartbeat),
	}
	if err := agentconfig.Save(e.host.Path(agentconfig.Path), cfg); err != nil {
		return fmt.Errorf("node %s registered but saving its config failed: %w", resp.Node.ID, err)
	}

	fmt.Fprintf(e.out, "node joined\n  id:     %s\n  name:   %s\n  subnet: %s\n  config: %s\n",
		resp.Node.ID, resp.Node.Name, resp.Node.OverlaySubnet, agentconfig.Path)
	if !e.host.UnitInstalled(host.AgentUnit) {
		fmt.Fprintln(e.out, "\nnext: sudo atellar node install && sudo systemctl start "+host.AgentUnit)
		return nil
	}
	return e.host.Restart(host.AgentUnit)
}

func (e *env) nodeRenewKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "renew-key",
		Short: "Replace the node API key now (the agent also renews it on its own)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			path := e.host.Path(agentconfig.Path)
			cfg, err := agentconfig.Load(path)
			if err != nil {
				return err
			}
			tlsConfig, err := cfg.TLS()
			if err != nil {
				return err
			}
			cl, err := client.New(client.Config{ControlPlane: cfg.ControlPlane, Transport: cfg.Transport, Token: client.Static(cfg.NodeAPIKey), TLS: tlsConfig})
			if err != nil {
				return err
			}
			defer cl.Close()
			key, err := cl.Nodes().SelfRenewAPIKey(c.Context())
			if err != nil {
				return err
			}
			cfg.NodeAPIKey, cfg.APIKeyExpiresAt = key.NodeAPIKey, key.APIKeyExpiresAt
			if err := agentconfig.Save(path, cfg); err != nil {
				return fmt.Errorf("key renewed but saving %s failed: %w", agentconfig.Path, err)
			}
			fmt.Fprintf(e.out, "api key renewed, expires %s\n", key.APIKeyExpiresAt.Format(time.RFC3339))
			// The running agent still holds the old, now revoked key.
			if e.host.UnitInstalled(host.AgentUnit) {
				return e.host.Restart(host.AgentUnit)
			}
			return nil
		},
	}
}

// nodeTrustCACmd stores the cluster CA in an existing agent config. Nodes
// joined before the API server served TLS use it once to upgrade.
func (e *env) nodeTrustCACmd() *cobra.Command {
	var ca caSource
	cmd := &cobra.Command{
		Use:   "trust-ca",
		Short: "Store the cluster CA in " + agentconfig.Path + " and restart the agent",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			path := e.host.Path(agentconfig.Path)
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var cfg agentconfig.Config
			if err := json.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("parse %s: %w", agentconfig.Path, err)
			}
			caPEM, err := ca.resolve(c.Context(), cfg.ControlPlane.Address, cfg.ControlPlane.HTTPPort)
			if err != nil {
				return err
			}
			cfg.CACert = string(caPEM)
			cfg.SetDefaults()
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := agentconfig.Save(path, &cfg); err != nil {
				return err
			}
			hash, _ := pki.HashPEM(caPEM)
			fmt.Fprintf(e.out, "cluster CA %s saved to %s\n", hash, agentconfig.Path)
			if e.host.UnitInstalled(host.AgentUnit) {
				return e.host.Restart(host.AgentUnit)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ca.hash, "ca-cert-hash", "", "pin of the cluster CA (sha256:...)")
	cmd.Flags().StringVar(&ca.file, "ca-cert", "", "cluster CA certificate file, instead of fetching it")
	return cmd
}
