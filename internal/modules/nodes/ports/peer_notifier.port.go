package ports

import (
	"context"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

type PeerNotifier interface {
	NotifyNodeAdded(ctx context.Context, newNode node.NodeEntity) error
	NotifyNodeRemoved(ctx context.Context, removedNode node.NodeEntity) error
	NotifyNodeUpdated(ctx context.Context, updatedNode node.NodeEntity, previousOverlayIP, previousOverlaySubnet string) error
}
