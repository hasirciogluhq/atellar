// Package informer keeps a local copy of a watched collection. It opens a
// watch, replaces its cache with each snapshot, applies live events, and
// reconnects with backoff when the watch ends.
package informer

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
)

// OpenFunc opens one watch.
type OpenFunc[T any] func(ctx context.Context) (watch.Interface[T], error)

type Config[T any] struct {
	// Name labels log lines.
	Name string
	Open OpenFunc[T]
	// Key identifies an object, usually its ID.
	Key func(T) string
	// OnChange receives the full cache, sorted by key, after the first
	// snapshot and after every later event. It runs on the informer
	// goroutine and must not block for long.
	OnChange func([]T)

	MinBackoff time.Duration // default 500ms
	MaxBackoff time.Duration // default 30s
}

type Informer[T any] struct {
	cfg Config[T]

	mu     sync.RWMutex
	cache  map[string]T
	synced bool
}

func New[T any](cfg Config[T]) *Informer[T] {
	if cfg.MinBackoff <= 0 {
		cfg.MinBackoff = 500 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 30 * time.Second
	}
	return &Informer[T]{cfg: cfg, cache: map[string]T{}}
}

// Synced reports whether a snapshot has been received since start.
func (i *Informer[T]) Synced() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.synced
}

// Get returns one cached object.
func (i *Informer[T]) Get(key string) (T, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	obj, ok := i.cache[key]
	return obj, ok
}

// List returns the cache sorted by key.
func (i *Informer[T]) List() []T {
	i.mu.RLock()
	defer i.mu.RUnlock()
	keys := make([]string, 0, len(i.cache))
	for k := range i.cache {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(keys))
	for _, k := range keys {
		out = append(out, i.cache[k])
	}
	return out
}

// Run watches until ctx is done.
func (i *Informer[T]) Run(ctx context.Context) {
	backoff := i.cfg.MinBackoff
	for ctx.Err() == nil {
		synced, err := i.watchOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if synced {
			backoff = i.cfg.MinBackoff
		}
		log.Printf("informer %s: watch ended: %v; reconnecting in %s", i.cfg.Name, err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, i.cfg.MaxBackoff)
	}
}

// watchOnce runs one watch to its end and reports whether it delivered a
// snapshot.
func (i *Informer[T]) watchOnce(ctx context.Context) (bool, error) {
	w, err := i.cfg.Open(ctx)
	if err != nil {
		return false, err
	}
	defer w.Stop()

	snapshot := map[string]T{}
	synced := false
	for {
		select {
		case <-ctx.Done():
			return synced, ctx.Err()
		case ev, ok := <-w.ResultChan():
			if !ok {
				return synced, w.Err()
			}
			if !synced {
				switch ev.Type {
				case watch.Bookmark:
					synced = true
					i.replace(snapshot)
				case watch.Deleted:
					delete(snapshot, i.cfg.Key(ev.Object))
				default:
					snapshot[i.cfg.Key(ev.Object)] = ev.Object
				}
				continue
			}
			i.apply(ev)
		}
	}
}

func (i *Informer[T]) replace(snapshot map[string]T) {
	i.mu.Lock()
	i.cache = snapshot
	i.synced = true
	i.mu.Unlock()
	i.notify()
}

func (i *Informer[T]) apply(ev watch.Event[T]) {
	if ev.Type == watch.Bookmark {
		return
	}
	key := i.cfg.Key(ev.Object)
	i.mu.Lock()
	if ev.Type == watch.Deleted {
		delete(i.cache, key)
	} else {
		i.cache[key] = ev.Object
	}
	i.mu.Unlock()
	i.notify()
}

func (i *Informer[T]) notify() {
	if i.cfg.OnChange != nil {
		i.cfg.OnChange(i.List())
	}
}
