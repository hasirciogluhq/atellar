package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
)

// streamEvents prints watch events, one per line, until ctx is done or the
// watch ends. The initial snapshot ends with a BOOKMARK line.
func streamEvents[T any](ctx context.Context, e *env, ch <-chan watch.Event[T], errFn func() error, describe func(T) string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return errFn()
			}
			if e.output == "json" {
				data, err := json.Marshal(ev)
				if err != nil {
					return err
				}
				fmt.Fprintln(e.out, string(data))
				continue
			}
			if ev.Type == watch.Bookmark {
				fmt.Fprintln(e.out, "BOOKMARK")
				continue
			}
			fmt.Fprintf(e.out, "%-8s %s\n", ev.Type, describe(ev.Object))
		}
	}
}
