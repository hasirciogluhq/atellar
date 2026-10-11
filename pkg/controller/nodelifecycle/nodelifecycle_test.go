package nodelifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/events"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/memory"
)

// TestSync covers status x heartbeat age.
func TestSync(t *testing.T) {
	statuses := []v1.NodeStatus{
		v1.NodeStatusPending, v1.NodeStatusReady, v1.NodeStatusNotReady,
		v1.NodeStatusMaintenance, v1.NodeStatusEvicted,
	}
	ages := []struct {
		name      string
		heartbeat bool
		age       time.Duration
		stale     bool
	}{
		{"fresh heartbeat", true, 5 * time.Second, false},
		{"old heartbeat", true, time.Minute, true},
		{"never, young", false, 5 * time.Second, false},
		{"never, old", false, time.Minute, true},
	}
	for _, status := range statuses {
		for _, age := range ages {
			t.Run(string(status)+"/"+age.name, func(t *testing.T) {
				ctx := context.Background()
				store := memory.New()
				start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				now := start
				store.SetClock(func() time.Time { return now })

				n, err := store.CreateNode(ctx, storage.CreateNodeParams{Name: "n"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.SetNodeStatus(ctx, n.ID, status); err != nil {
					t.Fatal(err)
				}
				if age.heartbeat {
					now = start.Add(time.Hour)
					if _, err := store.HeartbeatNode(ctx, n.ID); err != nil {
						t.Fatal(err)
					}
				}
				now = now.Add(age.age)

				broker := events.NewBroker[v1.Node]()
				sub := broker.Subscribe(nil)
				defer sub.Cancel()
				c := New(store, broker, 30*time.Second, 0)
				c.now = func() time.Time { return now }

				changed, err := c.Sync(ctx)
				if err != nil {
					t.Fatal(err)
				}
				want := age.stale && status == v1.NodeStatusReady
				if (len(changed) == 1) != want {
					t.Fatalf("changed %d nodes, want change=%v", len(changed), want)
				}
				got, _ := store.GetNode(ctx, n.ID)
				wantStatus := status
				if want {
					wantStatus = v1.NodeStatusNotReady
				}
				if got.Status != wantStatus {
					t.Fatalf("status %s, want %s", got.Status, wantStatus)
				}
				select {
				case ev := <-sub.C():
					if !want || ev.Type != watch.Modified || ev.Object.Status != v1.NodeStatusNotReady {
						t.Fatalf("unexpected event %s %s", ev.Type, ev.Object.Status)
					}
				default:
					if want {
						t.Fatal("no event published")
					}
				}
			})
		}
	}
}
