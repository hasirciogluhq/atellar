package containerd

import (
	"context"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

func TestResourceOpts(t *testing.T) {
	cpu, mem := 1.5, int32(256)
	cases := []struct {
		name   string
		c      v1.Container
		quota  int64
		shares uint64
		memory int64
	}{
		{"none", v1.Container{}, 0, 0, 0},
		{"all", v1.Container{CpuLimit: &cpu, CpuShares: 512, MemoryLimitMiB: &mem}, 150_000, 512, 256 << 20},
		{"shares only", v1.Container{CpuShares: 1024}, 0, 1024, 0},
	}
	for _, tc := range cases {
		spec := &specs.Spec{Linux: &specs.Linux{}}
		for _, opt := range resourceOpts(&tc.c) {
			if err := opt(context.Background(), nil, nil, spec); err != nil {
				t.Fatal(err)
			}
		}
		r := spec.Linux.Resources
		var quota, memory int64
		var shares uint64
		if r != nil && r.CPU != nil {
			if r.CPU.Quota != nil {
				quota = *r.CPU.Quota
				if *r.CPU.Period != cpuPeriod {
					t.Errorf("%s: period %d", tc.name, *r.CPU.Period)
				}
			}
			if r.CPU.Shares != nil {
				shares = *r.CPU.Shares
			}
		}
		if r != nil && r.Memory != nil && r.Memory.Limit != nil {
			memory = *r.Memory.Limit
		}
		if quota != tc.quota || shares != tc.shares || memory != tc.memory {
			t.Errorf("%s: quota %d shares %d memory %d", tc.name, quota, shares, memory)
		}
	}
}
