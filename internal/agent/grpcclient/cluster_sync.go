package grpcclient

import (
	"context"
	"net"

	"github.com/hasirciogluhq/atellar/internal/agent/overlay"
	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
)

func (s *Session) SyncClusterState(ctx context.Context) ([]overlay.ClusterNode, []overlay.ClusterContainer, error) {
	syncCtx := authn.OutgoingContext(ctx, authn.Credential{
		Type:  authn.CredentialTypeNodeAPIKey,
		Value: s.CurrentAPIKey(),
	})

	resp, err := s.client.GetClusterNetworkState(syncCtx, &atellarv1.GetClusterNetworkStateRequest{})
	if err != nil {
		return nil, nil, err
	}

	nodes := make([]overlay.ClusterNode, 0, len(resp.GetNodes()))
	for _, item := range resp.GetNodes() {
		nodes = append(nodes, overlay.ClusterNode{
			ID:            item.GetId(),
			OverlayIP:     net.ParseIP(item.GetOverlayIp()),
			OverlaySubnet: item.GetOverlaySubnet(),
			Status:        item.GetStatus(),
		})
	}

	containers := make([]overlay.ClusterContainer, 0, len(resp.GetContainers()))
	for _, item := range resp.GetContainers() {
		containers = append(containers, overlay.ClusterContainer{
			ID:        item.GetId(),
			NodeID:    item.GetNodeId(),
			OverlayIP: net.ParseIP(item.GetOverlayIp()),
			Status:    item.GetStatus(),
		})
	}

	return nodes, containers, nil
}
