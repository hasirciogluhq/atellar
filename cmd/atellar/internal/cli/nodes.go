package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

func nodeRow(n v1.Node) []string {
	hb := ""
	if n.LastHeartbeat != nil {
		hb = n.LastHeartbeat.Format("2006-01-02T15:04:05Z07:00")
	}
	return []string{n.ID, n.Name, string(n.Status), n.PrivateIP, n.OverlaySubnet, hb}
}

var nodeHeader = []string{"ID", "NAME", "STATUS", "PRIVATE_IP", "SUBNET", "LAST_HEARTBEAT"}

func (e *env) nodesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "nodes", Short: "Inspect and evict nodes"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List nodes",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			nodes, err := cl.Nodes().List(c.Context())
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(nodes))
			for _, n := range nodes {
				rows = append(rows, nodeRow(n))
			}
			return e.print(nodes, nodeHeader, rows)
		},
	}

	get := &cobra.Command{
		Use:   "get NODE",
		Short: "Show one node by ID or name",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			n, err := cl.Nodes().Get(c.Context(), args[0])
			if err != nil {
				return err
			}
			return e.print(n, nodeHeader, [][]string{nodeRow(*n)})
		},
	}

	evict := &cobra.Command{
		Use:   "evict NODE",
		Short: "Remove a node from the cluster and revoke its key",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			n, err := cl.Nodes().Evict(c.Context(), args[0])
			if err != nil {
				return err
			}
			return e.print(n, nodeHeader, [][]string{nodeRow(*n)})
		},
	}

	var free bool
	ips := &cobra.Command{
		Use:   "overlay-ips NODE",
		Short: "List a node's container address pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			pool, err := cl.Nodes().ListOverlayIPs(c.Context(), args[0], free)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(pool))
			for _, ip := range pool {
				rows = append(rows, []string{ip.IP, or(str(ip.ContainerID), "-")})
			}
			return e.print(pool, []string{"IP", "CONTAINER"}, rows)
		},
	}
	ips.Flags().BoolVar(&free, "free", false, "only unallocated addresses")

	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream node changes until interrupted",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			w, err := cl.Nodes().Watch(c.Context())
			if err != nil {
				return err
			}
			defer w.Stop()
			return streamEvents(c.Context(), e, w.ResultChan(), w.Err, func(n v1.Node) string {
				return fmt.Sprintf("%s %s %s", n.ID, n.Name, n.Status)
			})
		},
	}

	cmd.AddCommand(list, get, evict, ips, watchCmd)
	return cmd
}
