//go:build linux

package hardware

import (
	"runtime"
	"testing"
)

func TestCollect(t *testing.T) {
	info, err := Collect()
	if err != nil {
		t.Fatal(err)
	}
	if info.CpuCores != int32(runtime.NumCPU()) {
		t.Errorf("CpuCores = %d, want %d", info.CpuCores, runtime.NumCPU())
	}
	if info.MemoryTotalMiB <= 0 {
		t.Errorf("MemoryTotalMiB = %d", info.MemoryTotalMiB)
	}
	if info.OS != "linux" || info.Arch != runtime.GOARCH || info.KernelVersion == "" {
		t.Errorf("unexpected platform %+v", info)
	}
}
