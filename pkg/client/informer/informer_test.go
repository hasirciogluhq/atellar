package informer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
)

type item struct{ ID, V string }

// script feeds each opened watch from a list of event batches; a nil batch
// fails the open.
type script struct {
	mu      sync.Mutex
	batches [][]watch.Event[item]
	opened  int
	streams []*watch.Stream[item]
}

func (s *script) open(ctx context.Context) (watch.Interface[item], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opened >= len(s.batches) {
		// Park forever: the test is done reconnecting.
		st := watch.NewStream[item](ctx, 1)
		s.streams = append(s.streams, st)
		return st, nil
	}
	batch := s.batches[s.opened]
	s.opened++
	if batch == nil {
		return nil, errors.New("connection refused")
	}
	st := watch.NewStream[item](ctx, len(batch)+1)
	for _, ev := range batch {
		st.Send(ev)
	}
	st.Close(errors.New("server went away"))
	return st, nil
}

func ev(t watch.EventType, id, v string) watch.Event[item] {
	return watch.Event[item]{Type: t, Object: item{ID: id, V: v}}
}

func TestInformer(t *testing.T) {
	bookmark := watch.Event[item]{Type: watch.Bookmark}
	s := &script{batches: [][]watch.Event[item]{
		nil, // first connect fails
		{ev(watch.Added, "a", "1"), ev(watch.Added, "b", "1"), bookmark, ev(watch.Modified, "a", "2"), ev(watch.Deleted, "b", "")},
		// Dropped before the snapshot finished: the cache must not change.
		{ev(watch.Added, "zzz", "partial")},
		// Reconnect: "a" vanished while disconnected, "c" appeared.
		{ev(watch.Added, "c", "1"), bookmark},
	}}

	var mu sync.Mutex
	var seen [][]item
	inf := New(Config[item]{
		Name: "test", Open: s.open, Key: func(i item) string { return i.ID },
		OnChange: func(items []item) {
			mu.Lock()
			seen = append(seen, items)
			mu.Unlock()
		},
		MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond,
	})
	if inf.Synced() {
		t.Fatal("synced before start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { inf.Run(ctx); close(done) }()

	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		n := len(seen)
		mu.Unlock()
		if n >= 4 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("got %d notifications", n)
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	<-done

	want := [][]item{
		{{"a", "1"}, {"b", "1"}},
		{{"a", "2"}, {"b", "1"}},
		{{"a", "2"}},
		{{"c", "1"}},
	}
	mu.Lock()
	defer mu.Unlock()
	for i, w := range want {
		if len(seen[i]) != len(w) {
			t.Fatalf("notification %d = %v, want %v", i, seen[i], w)
		}
		for j := range w {
			if seen[i][j] != w[j] {
				t.Fatalf("notification %d = %v, want %v", i, seen[i], w)
			}
		}
	}
	if !inf.Synced() {
		t.Fatal("not synced")
	}
	if got, ok := inf.Get("c"); !ok || got.V != "1" {
		t.Fatalf("Get(c) = %v %v", got, ok)
	}
	if _, ok := inf.Get("a"); ok {
		t.Fatal("stale object kept after resync")
	}
}
