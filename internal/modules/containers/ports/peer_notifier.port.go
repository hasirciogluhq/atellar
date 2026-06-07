package ports

import (
	"context"

	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
)

type PeerNotifier interface {
	NotifyContainerEvent(ctx context.Context, event string, target container.Entity) error
	DispatchWorkload(ctx context.Context, target container.Entity) error
	RemoveWorkload(ctx context.Context, target container.Entity) error
}
