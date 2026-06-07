package agentregistry

import (
	"context"
	"log"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

type PeerNotifier struct {
	registry *Registry
}

func NewPeerNotifier(registry *Registry) *PeerNotifier {
	return &PeerNotifier{registry: registry}
}

func (n *PeerNotifier) NotifyNodeAdded(ctx context.Context, newNode node.NodeEntity) error {
	_ = ctx

	notified := n.registry.NotifyNodeAdded(newNode)
	log.Printf("node.added notified peers=%d new_node_id=%s", notified, newNode.ID)
	return nil
}

func (n *PeerNotifier) NotifyNodeRemoved(ctx context.Context, removedNode node.NodeEntity) error {
	_ = ctx

	notified := n.registry.NotifyNodeRemoved(removedNode)
	log.Printf("node.removed notified peers=%d removed_node_id=%s", notified, removedNode.ID)
	return nil
}

func (n *PeerNotifier) NotifyNodeUpdated(ctx context.Context, updatedNode node.NodeEntity, previousOverlayIP, previousOverlaySubnet string) error {
	_ = ctx

	notified := n.registry.NotifyNodeUpdated(updatedNode, previousOverlayIP, previousOverlaySubnet)
	log.Printf("node.updated notified peers=%d node_id=%s", notified, updatedNode.ID)
	return nil
}

func (n *PeerNotifier) NotifyContainerEvent(ctx context.Context, event string, target container.Entity) error {
	_ = ctx

	notified := n.registry.NotifyContainerEvent(event, target)
	log.Printf("%s notified peers=%d container_id=%s node_id=%s", event, notified, target.ID, target.NodeID)
	return nil
}
