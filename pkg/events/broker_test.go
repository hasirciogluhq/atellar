package events

import (
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
)

func TestBrokerFiltersAndDelivers(t *testing.T) {
	b := NewBroker[int]()
	even := b.Subscribe(func(v int) bool { return v%2 == 0 })
	all := b.Subscribe(nil)
	defer even.Cancel()
	defer all.Cancel()

	for i := 1; i <= 4; i++ {
		b.Publish(watch.Added, i)
	}

	if got := drain(all.C(), 4); len(got) != 4 {
		t.Fatalf("all subscriber got %v, want 4 events", got)
	}
	got := drain(even.C(), 2)
	if len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Fatalf("even subscriber got %v, want [2 4]", got)
	}
}

func TestBrokerDropsSlowSubscriber(t *testing.T) {
	b := NewBroker[int]()
	b.buffer = 2
	slow := b.Subscribe(nil)

	for i := 0; i < 3; i++ {
		b.Publish(watch.Added, i)
	}

	if b.Len() != 0 {
		t.Fatalf("slow subscriber still registered")
	}
	n := 0
	for range slow.C() {
		n++
	}
	if n != 2 {
		t.Fatalf("slow subscriber received %d buffered events before close, want 2", n)
	}
	slow.Cancel() // must be safe after a drop
}

func drain(ch <-chan watch.Event[int], n int) []int {
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		select {
		case ev := <-ch:
			out = append(out, ev.Object)
		default:
			return out
		}
	}
	return out
}
