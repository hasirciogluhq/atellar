// Package events is the in-process event bus of the API server. Every state
// change is published here and fanned out to open watches.
package events

import (
	"sync"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
)

// DefaultBuffer is the per-subscriber queue length. A subscriber that falls
// this far behind is dropped; its client reconnects and gets a fresh snapshot.
const DefaultBuffer = 256

// Broker fans out events of one object type to subscribers.
type Broker[T any] struct {
	mu     sync.RWMutex
	subs   map[*Subscription[T]]struct{}
	buffer int
}

func NewBroker[T any]() *Broker[T] {
	return &Broker[T]{subs: make(map[*Subscription[T]]struct{}), buffer: DefaultBuffer}
}

// Subscription receives events accepted by its filter.
type Subscription[T any] struct {
	broker *Broker[T]
	filter func(T) bool
	ch     chan watch.Event[T]
	once   sync.Once
}

// C is closed when the subscription is cancelled or dropped for being slow.
func (s *Subscription[T]) C() <-chan watch.Event[T] { return s.ch }

func (s *Subscription[T]) Cancel() {
	s.broker.mu.Lock()
	defer s.broker.mu.Unlock()
	s.closeLocked()
}

func (s *Subscription[T]) closeLocked() {
	s.once.Do(func() {
		delete(s.broker.subs, s)
		close(s.ch)
	})
}

// Subscribe registers a subscriber. A nil filter accepts everything.
func (b *Broker[T]) Subscribe(filter func(T) bool) *Subscription[T] {
	sub := &Subscription[T]{broker: b, filter: filter, ch: make(chan watch.Event[T], b.buffer)}
	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub
}

// Publish never blocks: subscribers whose queue is full are dropped.
func (b *Broker[T]) Publish(eventType watch.EventType, obj T) {
	ev := watch.Event[T]{Type: eventType, Object: obj}

	b.mu.RLock()
	var slow []*Subscription[T]
	for sub := range b.subs {
		if sub.filter != nil && !sub.filter(obj) {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
			slow = append(slow, sub)
		}
	}
	b.mu.RUnlock()

	if len(slow) == 0 {
		return
	}
	b.mu.Lock()
	for _, sub := range slow {
		sub.closeLocked()
	}
	b.mu.Unlock()
}

// Len returns the number of live subscribers.
func (b *Broker[T]) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
