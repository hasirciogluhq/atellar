//go:build !linux

package hardware

import "errors"

type Info struct {
	CpuCores       int32
	MemoryTotalMiB int32
	DiskTotalGiB   int32
	Hostname       string
	OS             string
	Arch           string
	KernelVersion  string
}

func Collect() (Info, error) {
	return Info{}, errors.New("hardware collection is only supported on linux")
}
