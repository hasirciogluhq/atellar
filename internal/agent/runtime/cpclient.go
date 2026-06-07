package runtime

import (
	"context"
	"fmt"
	"time"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
	"google.golang.org/grpc"
)

type CPClient struct {
	client atellarv1.AgentServiceClient
	apiKey string
}

func NewCPClient(client atellarv1.AgentServiceClient, apiKey string) *CPClient {
	return &CPClient{client: client, apiKey: apiKey}
}

func (c *CPClient) ctx(ctx context.Context) context.Context {
	return authn.OutgoingContext(ctx, authn.Credential{
		Type:  authn.CredentialTypeNodeAPIKey,
		Value: c.apiKey,
	})
}

func (c *CPClient) ListWorkloads(ctx context.Context) ([]Workload, error) {
	resp, err := c.client.GetNodeWorkloads(c.ctx(ctx), &atellarv1.GetNodeWorkloadsRequest{}, grpc.WaitForReady(true))
	if err != nil {
		return nil, err
	}

	out := make([]Workload, 0, len(resp.GetWorkloads()))
	for _, w := range resp.GetWorkloads() {
		out = append(out, Workload{
			ID:               w.GetId(),
			Image:            w.GetImage(),
			Command:          w.GetCommand(),
			Entrypoint:       w.GetEntrypoint(),
			Env:              w.GetEnv(),
			WorkingDir:       w.GetWorkingDir(),
			ContainerdNs:     w.GetContainerdNs(),
			Status:           w.GetStatus(),
			RestartPolicy:    w.GetRestartPolicy(),
			OverlayIP:        w.GetOverlayIp(),
			RestartCount:     w.GetRestartCount(),
			CpuLimit:         w.GetCpuLimit(),
			CpuShares:        w.GetCpuShares(),
			MemoryLimitMiB:   w.GetMemoryLimitMib(),
			ImageDigest:      w.GetImageDigest(),
			LastFailedAtUnix: w.GetLastFailedAtUnix(),
			ErrorMessage:     w.GetErrorMessage(),
		})
	}
	return out, nil
}

type RuntimeReport struct {
	ContainerID      string
	ContainerdID     string
	SnapshotKey      string
	TaskPID          int32
	ImageDigest      string
	OverlayIP        string
	Status           string
	ExitCode         int32
	ErrorMessage     string
	RestartCount     int32
	LastFailedAtUnix int64
}

func (c *CPClient) ReportRuntime(ctx context.Context, report RuntimeReport) error {
	_, err := c.client.ReportContainerRuntime(c.ctx(ctx), &atellarv1.ReportContainerRuntimeRequest{
		ContainerId:      report.ContainerID,
		ContainerdId:       report.ContainerdID,
		SnapshotKey:      report.SnapshotKey,
		TaskPid:          report.TaskPID,
		ImageDigest:      report.ImageDigest,
		OverlayIp:        report.OverlayIP,
		Status:           report.Status,
		ExitCode:         report.ExitCode,
		ErrorMessage:     report.ErrorMessage,
		RestartCount:     report.RestartCount,
		LastFailedAtUnix: report.LastFailedAtUnix,
	}, grpc.WaitForReady(true))
	return err
}

func (c *CPClient) AllocateOverlayIP(ctx context.Context, containerID string) (string, error) {
	resp, err := c.client.AllocateContainerOverlayIP(c.ctx(ctx), &atellarv1.AllocateContainerOverlayIPRequest{
		ContainerId: containerID,
	}, grpc.WaitForReady(true))
	if err != nil {
		return "", err
	}
	if resp.GetOverlayIp() == "" {
		return "", fmt.Errorf("empty overlay ip")
	}
	return resp.GetOverlayIp(), nil
}

func (c *CPClient) ReportHardware(ctx context.Context, req *atellarv1.ReportNodeHardwareRequest) error {
	_, err := c.client.ReportNodeHardware(c.ctx(ctx), req, grpc.WaitForReady(true))
	return err
}

func NowUnix() int64 {
	return time.Now().Unix()
}
