package transport_test

import (
	"context"
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

// BenchmarkNodesGet measures one authenticated unary call over loopback,
// including authentication, authorization and an in-memory store read.
func BenchmarkNodesGet(b *testing.B) {
	for _, kind := range []transport.Kind{transport.HTTP, transport.GRPC} {
		b.Run(string(kind), func(b *testing.B) {
			f := apiservertest.New(b)
			c := serve(b, kind, f)(apiservertest.AdminToken)
			ctx := context.Background()
			if _, err := c.Nodes().Get(ctx, f.NodeA.ID); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := c.Nodes().Get(ctx, f.NodeA.ID); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkNodesGetParallel is the same call from GOMAXPROCS goroutines
// sharing one client.
func BenchmarkNodesGetParallel(b *testing.B) {
	for _, kind := range []transport.Kind{transport.HTTP, transport.GRPC} {
		b.Run(string(kind), func(b *testing.B) {
			f := apiservertest.New(b)
			c := serve(b, kind, f)(apiservertest.AdminToken)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				ctx := context.Background()
				for pb.Next() {
					if _, err := c.Nodes().Get(ctx, f.NodeA.ID); err != nil {
						b.Error(err)
						return
					}
				}
			})
		})
	}
}
