package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/hasirciogluhq/atellar/internal/atelctl/cluster"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/spf13/cobra"
)

var (
	clusterControlPlaneAddress string
	clusterHTTPPort            int
	clusterGRPCPort            int
)

func clusterControlPlane() client.ControlPlane {
	return client.ControlPlane{
		Address:  clusterControlPlaneAddress,
		HTTPPort: clusterHTTPPort,
		GRPCPort: clusterGRPCPort,
	}
}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Control plane cluster operations",
}

var clusterNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Manage cluster nodes",
}

var clusterContainersCmd = &cobra.Command{
	Use:   "containers",
	Short: "Manage cluster containers",
}

var clusterNodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		nodes, err := cluster.ListNodes(context.Background(), clusterControlPlane())
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tOVERLAY_IP\tOVERLAY_SUBNET")
		for _, n := range nodes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				n.ID, n.Name, n.Status, n.OverlayIP, n.OverlaySubnet)
		}
		return w.Flush()
	},
}

var clusterContainersListNodeID string

var clusterContainersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		containers, err := cluster.ListContainers(context.Background(), clusterControlPlane(), clusterContainersListNodeID)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNODE_ID\tIMAGE\tSTATUS\tOVERLAY_IP\tERROR")
		for _, c := range containers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				c.ID, c.NodeID, c.Image, c.Status, c.OverlayIP, c.ErrorMessage)
		}
		return w.Flush()
	},
}

func init() {
	clusterCmd.PersistentFlags().StringVar(&clusterControlPlaneAddress, "control-plane-address", "", "control plane host or IP")
	clusterCmd.PersistentFlags().IntVar(&clusterHTTPPort, "http-port", 0, "control plane HTTP port")
	clusterCmd.PersistentFlags().IntVar(&clusterGRPCPort, "grpc-port", 0, "control plane gRPC port")

	for _, flag := range []string{"control-plane-address", "http-port", "grpc-port"} {
		_ = clusterCmd.MarkPersistentFlagRequired(flag)
	}

	clusterContainersListCmd.Flags().StringVar(&clusterContainersListNodeID, "node-id", "", "filter by node id")

	clusterNodesCmd.AddCommand(clusterNodesListCmd)
	clusterContainersCmd.AddCommand(clusterContainersListCmd)
	clusterCmd.AddCommand(clusterNodesCmd, clusterContainersCmd)
	rootCmd.AddCommand(clusterCmd)
}
