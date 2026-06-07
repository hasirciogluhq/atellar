package hardware

import (
	"context"
	"log"
	"time"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
)

const reportInterval = 10 * time.Minute

type Reporter struct {
	client   atellarv1.AgentServiceClient
	cpReport func(ctx context.Context, req *atellarv1.ReportNodeHardwareRequest) error
	interval time.Duration
}

func NewReporter(client atellarv1.AgentServiceClient, report func(ctx context.Context, req *atellarv1.ReportNodeHardwareRequest) error) *Reporter {
	return &Reporter{client: client, cpReport: report, interval: reportInterval}
}

func (r *Reporter) Run(ctx context.Context) {
	r.reportOnce(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reportOnce(ctx)
		}
	}
}

func (r *Reporter) reportOnce(ctx context.Context) {
	info, err := Collect()
	if err != nil {
		log.Printf("hardware collect failed: %v", err)
		return
	}

	err = r.cpReport(ctx, &atellarv1.ReportNodeHardwareRequest{
		CpuCores:       info.CpuCores,
		MemoryTotalMib: info.MemoryTotalMiB,
		DiskTotalGib:   info.DiskTotalGiB,
		Hostname:       info.Hostname,
		Os:             info.OS,
		Arch:           info.Arch,
		KernelVersion:  info.KernelVersion,
	})
	if err != nil {
		log.Printf("hardware report failed: %v", err)
		return
	}
	log.Printf("hardware reported cpu=%d mem_mib=%d", info.CpuCores, info.MemoryTotalMiB)
}
