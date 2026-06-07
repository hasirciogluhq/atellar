package cluster

import (
	"context"

	"github.com/hasirciogluhq/atellar/pkg/client"
)

type Node struct {
	ID            string
	Name          string
	Status        string
	OverlayIP     string
	OverlaySubnet string
}

type Container struct {
	ID           string
	NodeID       string
	Image        string
	Status       string
	OverlayIP    string
	ErrorMessage string
}

func ListNodes(ctx context.Context, cp client.ControlPlane) ([]Node, error) {
	if err := cp.Validate(); err != nil {
		return nil, err
	}

	nodes, err := client.New(client.Options{BaseURL: cp.HTTPBaseURL()}).ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Node{
			ID:            n.ID,
			Name:          n.Name,
			Status:        n.Status,
			OverlayIP:     n.OverlayIP,
			OverlaySubnet: n.OverlaySubnet,
		})
	}
	return out, nil
}

func ListContainers(ctx context.Context, cp client.ControlPlane, nodeID string) ([]Container, error) {
	if err := cp.Validate(); err != nil {
		return nil, err
	}

	containers, err := client.New(client.Options{BaseURL: cp.HTTPBaseURL()}).ListContainers(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	out := make([]Container, 0, len(containers))
	for _, c := range containers {
		out = append(out, Container{
			ID:           c.ID,
			NodeID:       c.NodeID,
			Image:        c.Image,
			Status:       c.Status,
			OverlayIP:    c.OverlayIP,
			ErrorMessage: c.ErrorMessage,
		})
	}
	return out, nil
}
