package scheduler

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

type fakeNode struct {
	cores, memMiB int32
	usage         storage.ResourceUsage
	freeIPs       int
	images        map[string]bool
}

type fakeStore map[string]*fakeNode

func (f fakeStore) ListSchedulableNodes(context.Context) ([]v1.Node, error) {
	out := []v1.Node{}
	for _, id := range []string{"node_a", "node_b", "node_c"} {
		n, ok := f[id]
		if !ok {
			continue
		}
		out = append(out, v1.Node{ID: id, CpuCores: &n.cores, MemoryTotalMiB: &n.memMiB})
	}
	return out, nil
}

func (f fakeStore) NodeResourceUsage(_ context.Context, id string) (*storage.ResourceUsage, error) {
	u := f[id].usage
	return &u, nil
}

func (f fakeStore) CountFreeOverlayIPs(_ context.Context, id string) (int, error) {
	return f[id].freeIPs, nil
}

func (f fakeStore) HasActiveContainerWithImage(_ context.Context, id, image string) (bool, error) {
	return f[id].images[image], nil
}

func ptr[T any](v T) *T { return &v }

func TestSelectNodePicksLeastLoaded(t *testing.T) {
	s := New(fakeStore{
		"node_a": {cores: 4, memMiB: 4096, usage: storage.ResourceUsage{RunningCount: 3}, freeIPs: 10},
		"node_b": {cores: 4, memMiB: 4096, usage: storage.ResourceUsage{RunningCount: 1}, freeIPs: 10},
	})
	got, err := s.SelectNode(context.Background(), Request{Image: "nginx"})
	if err != nil || got != "node_b" {
		t.Fatalf("got %q, %v; want node_b", got, err)
	}
}

func TestSelectNodeFilters(t *testing.T) {
	s := New(fakeStore{
		"node_a": {cores: 2, memMiB: 4096, usage: storage.ResourceUsage{TotalCPU: 1.5, RunningCount: 2}, freeIPs: 10}, // not enough CPU
		"node_b": {cores: 4, memMiB: 4096, freeIPs: 0},                                                                // no overlay IPs
		"node_c": {cores: 4, memMiB: 4096, freeIPs: 10, images: map[string]bool{"nginx": true}},                       // image already there
	})
	_, err := s.SelectNode(context.Background(), Request{Image: "nginx", CpuLimit: ptr(1.0)})
	if !errors.Is(err, ErrNoNodes) {
		t.Fatalf("err = %v, want ErrNoNodes", err)
	}

	got, err := s.SelectNode(context.Background(), Request{Image: "redis", MemoryLimitMiB: ptr(int32(512))})
	if err != nil || got != "node_c" {
		t.Fatalf("got %q, %v; want node_c", got, err)
	}
}
