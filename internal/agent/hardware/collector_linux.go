//go:build linux

package hardware

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

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
	host, _ := os.Hostname()
	mem, err := readMemTotalMiB()
	if err != nil {
		return Info{}, err
	}
	disk, err := readDiskTotalGiB("/")
	if err != nil {
		return Info{}, err
	}

	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return Info{}, err
	}

	return Info{
		CpuCores:       int32(runtime.NumCPU()),
		MemoryTotalMiB: mem,
		DiskTotalGiB:   disk,
		Hostname:       host,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		KernelVersion:  strings.TrimRight(unix.ByteSliceToString(uname.Release[:]), "\x00"),
	}, nil
}

func readMemTotalMiB() (int32, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return int32(kb / 1024), nil
		}
	}
	return 0, os.ErrNotExist
}

func readDiskTotalGiB(path string) (int32, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	return int32(totalBytes / (1024 * 1024 * 1024)), nil
}
