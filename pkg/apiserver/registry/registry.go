// Package registry holds what every API resource implementation shares:
// storage, the authorizer, the event brokers and error mapping.
package registry

import (
	"context"
	"errors"
	"log"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authz"
	"github.com/hasirciogluhq/atellar/pkg/events"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

// watchBuffer is how many events may queue between producer and transport.
const watchBuffer = 64

// Deps is shared by the node, container and join token registries.
type Deps struct {
	Store           storage.Interface
	Authz           *authz.Authorizer
	NodeEvents      *events.Broker[v1.Node]
	ContainerEvents *events.Broker[v1.Container]
}

// Authorize checks the caller in ctx and returns an api.Error on refusal.
func (d *Deps) Authorize(ctx context.Context, action authz.Action) error {
	err := d.Authz.Authorize(ctx, action)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, authz.ErrUnauthenticated):
		return api.Unauthorized("%v", err)
	default:
		return api.Forbidden("%v", err)
	}
}

// StorageError converts a storage error into an api.Error. kind and ref
// describe the object for NotFound messages.
func StorageError(err error, kind, ref string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, storage.ErrNotFound):
		return api.NotFound("%s %q not found", kind, ref)
	case errors.Is(err, storage.ErrConflict):
		return api.Conflict("%v", err)
	default:
		log.Printf("apiserver: %s %q: %v", kind, ref, err)
		return api.Internal("%s %q: internal error", kind, ref)
	}
}

// Watch subscribes first and lists second, so no change between the two is
// lost. It sends the list as ADDED events, a BOOKMARK, then live events. A
// watch whose consumer falls too far behind ends with ReasonUnavailable;
// clients reconnect and receive a fresh snapshot.
func Watch[T any](
	ctx context.Context,
	broker *events.Broker[T],
	filter func(T) bool,
	list func(context.Context) ([]T, error),
) (watch.Interface[T], error) {
	sub := broker.Subscribe(filter)
	items, err := list(ctx)
	if err != nil {
		sub.Cancel()
		return nil, err
	}

	stream := watch.NewStream[T](ctx, watchBuffer)
	go func() {
		defer sub.Cancel()
		for _, item := range items {
			if !stream.Send(watch.Event[T]{Type: watch.Added, Object: item}) {
				stream.Close(nil)
				return
			}
		}
		var zero T
		if !stream.Send(watch.Event[T]{Type: watch.Bookmark, Object: zero}) {
			stream.Close(nil)
			return
		}
		for {
			select {
			case <-stream.Context().Done():
				stream.Close(nil)
				return
			case ev, ok := <-sub.C():
				if !ok {
					stream.Close(api.Unavailable("watch dropped: consumer too slow"))
					return
				}
				if !stream.Send(ev) {
					stream.Close(nil)
					return
				}
			}
		}
	}()
	return stream, nil
}

// BadRequest wraps a validation error.
func BadRequest(err error) error {
	return api.BadRequest("%v", err)
}
