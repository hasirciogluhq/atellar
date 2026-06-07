package agentclient

import (
	"context"
	"net"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
)

func (s *Session) SyncClusterState(ctx context.Context) ([]node.NodeEntity, []container.Entity, error) {
	syncCtx := authn.OutgoingContext(ctx, authn.Credential{
		Type:  authn.CredentialTypeNodeAPIKey,
		Value: s.cfg.NodeAPIKey,
	})

	resp, err := s.client.GetClusterNetworkState(syncCtx, &atellarv1.GetClusterNetworkStateRequest{})
	if err != nil {
		return nil, nil, err
	}

	nodes := make([]node.NodeEntity, 0, len(resp.GetNodes()))
	for _, item := range resp.GetNodes() {
		nodes = append(nodes, node.NodeEntity{
			ID:            item.GetId(),
			OverlayIP:     net.ParseIP(item.GetOverlayIp()),
			OverlaySubnet: item.GetOverlaySubnet(),
			Status:        node.NodeStatus(item.GetStatus()),
		})
	}

	containers := make([]container.Entity, 0, len(resp.GetContainers()))
	for _, item := range resp.GetContainers() {
		containers = append(containers, container.Entity{
			ID:        item.GetId(),
			NodeID:    item.GetNodeId(),
			OverlayIP: net.ParseIP(item.GetOverlayIp()),
			Status:    container.Status(item.GetStatus()),
		})
	}

	return nodes, containers, nil
}
