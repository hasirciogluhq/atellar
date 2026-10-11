// Package watch is the streaming half of the API: a typed channel of change
// events that every transport can carry.
package watch

import (
	"context"
	"sync"
)

type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	// Bookmark marks the end of the initial snapshot. Its Object is zero.
	Bookmark EventType = "BOOKMARK"
)

type Event[T any] struct {
	Type   EventType `json:"type"`
	Object T         `json:"object"`
}

// Interface is an open watch. The result channel is closed when the watch
// ends (Stop, server side close, or connection loss); Err tells why.
type Interface[T any] interface {
	ResultChan() <-chan Event[T]
	Stop()
	Err() error
}

// Stream is a ready-made Interface for producers: a goroutine sends events
// with Send and finishes with Close.
type Stream[T any] struct {
	ch     chan Event[T]
	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	err  error
	once sync.Once
}

func NewStream[T any](ctx context.Context, buffer int) *Stream[T] {
	ctx, cancel := context.WithCancel(ctx)
	return &Stream[T]{ch: make(chan Event[T], buffer), ctx: ctx, cancel: cancel}
}

// Context is done once the consumer stopped the watch.
func (s *Stream[T]) Context() context.Context { return s.ctx }

// Send delivers ev, or returns false when the watch was stopped.
func (s *Stream[T]) Send(ev Event[T]) bool {
	select {
	case s.ch <- ev:
		return true
	case <-s.ctx.Done():
		return false
	}
}

// Close ends the stream with err (nil for a clean end). Only the producer
// may call it, and only once it stopped sending.
func (s *Stream[T]) Close(err error) {
	s.once.Do(func() {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.ch)
		s.cancel()
	})
}

func (s *Stream[T]) ResultChan() <-chan Event[T] { return s.ch }
func (s *Stream[T]) Stop()                       { s.cancel() }

func (s *Stream[T]) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
