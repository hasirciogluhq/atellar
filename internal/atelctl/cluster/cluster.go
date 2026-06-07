package cluster

import (
	"context"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/client/controlplane"
)

type Node struct {
	ID            string
	Name          string
	Status        string
	OverlayIP     string
	OverlaySubnet string
}

type Container struct {
	ID        string
	NodeID    string
	Image     string
	Status    string
	OverlayIP string
}

func ListNodes(ctx context.Context, controlPlaneURL string) ([]Node, error) {
	if controlPlaneURL == "" {
		return nil, fmt.Errorf("control plane url is required")
	}

	client := controlplane.NewClient(controlPlaneURL)
	nodes, err := client.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Node{
			ID:            n.ID,
			Name:          n.Name,
			Status:        string(n.Status),
			OverlayIP:     n.OverlayIP.String(),
			OverlaySubnet: n.OverlaySubnet,
		})
	}
	return out, nil
}

func ListContainers(ctx context.Context, controlPlaneURL, nodeID string) ([]Container, error) {
	if controlPlaneURL == "" {
		return nil, fmt.Errorf("control plane url is required")
	}

	client := controlplane.NewClient(controlPlaneURL)
	containers, err := client.ListContainers(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	out := make([]Container, 0, len(containers))
	for _, c := range containers {
		out = append(out, Container{
			ID:        c.ID,
			NodeID:    c.NodeID,
			Image:     c.Image,
			Status:    string(c.Status),
			OverlayIP: c.OverlayIP.String(),
		})
	}
	return out, nil
}
