package watch

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStreamDeliversThenCloses(t *testing.T) {
	s := NewStream[int](context.Background(), 2)
	if !s.Send(Event[int]{Type: Added, Object: 1}) || !s.Send(Event[int]{Type: Bookmark}) {
		t.Fatal("send failed on a live stream")
	}
	boom := errors.New("boom")
	s.Close(boom)
	s.Close(nil) // second close is a no-op

	var got []EventType
	for ev := range s.ResultChan() {
		got = append(got, ev.Type)
	}
	if len(got) != 2 || got[0] != Added || got[1] != Bookmark {
		t.Fatalf("events = %v", got)
	}
	if !errors.Is(s.Err(), boom) {
		t.Fatalf("Err = %v, want boom", s.Err())
	}
	if s.Context().Err() == nil {
		t.Fatal("context not cancelled after Close")
	}
}

func TestStreamStopUnblocksSender(t *testing.T) {
	s := NewStream[int](context.Background(), 0)
	done := make(chan bool)
	go func() { done <- s.Send(Event[int]{Type: Modified}) }()
	s.Stop()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("Send reported delivery after Stop")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send blocked after Stop")
	}
	if s.Context().Err() == nil {
		t.Fatal("context not cancelled by Stop")
	}
}

func TestStreamParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewStream[int](ctx, 0)
	cancel()
	if s.Send(Event[int]{Type: Deleted}) {
		t.Fatal("Send succeeded after parent cancel")
	}
}
