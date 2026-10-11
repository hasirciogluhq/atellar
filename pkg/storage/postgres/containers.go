package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/jackc/pgx/v5/pgtype"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/postgres/db"
)

// containerdNamespace is fixed; it is not part of the API.
const containerdNamespace = "atellar"

func (s *Store) CreateContainer(ctx context.Context, params storage.CreateContainerParams) (*v1.Container, error) {
	id, err := newID("ctr_")
	if err != nil {
		return nil, err
	}
	env := params.Env
	if env == nil {
		env = map[string]string{}
	}
	envJSON, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	restartPolicy := params.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = v1.RestartPolicyOnFailure
	}
	cpuShares := int32(1024)
	if params.CpuShares != nil {
		cpuShares = *params.CpuShares
	}

	row, err := s.q.CreateContainer(ctx, db.CreateContainerParams{
		ID:             id,
		NodeID:         params.NodeID,
		Image:          params.Image,
		Command:        params.Command,
		Entrypoint:     params.Entrypoint,
		Env:            envJSON,
		WorkingDir:     toText(params.WorkingDir),
		CpuLimit:       toNumeric(params.CpuLimit),
		CpuShares:      pgtype.Int4{Int32: cpuShares, Valid: true},
		MemoryLimitMib: toInt4(params.MemoryLimitMiB),
		RestartPolicy:  string(restartPolicy),
		ContainerdNs:   containerdNamespace,
		Status:         db.ContainerStatusPending,
	})
	if err != nil {
		return nil, wrap("create container", err)
	}
	return containerFromRow(row), nil
}

func (s *Store) GetContainer(ctx context.Context, id string) (*v1.Container, error) {
	row, err := s.q.GetContainerById(ctx, id)
	if err != nil {
		return nil, wrap("get container", err)
	}
	return containerFromRow(row), nil
}

func (s *Store) ListContainers(ctx context.Context, opts v1.ContainerListOptions) ([]v1.Container, error) {
	var (
		rows []db.Container
		err  error
	)
	switch {
	case opts.NodeID != "" && opts.ActiveOnly:
		rows, err = s.q.ListWorkloadsByNodeId(ctx, opts.NodeID)
	case opts.NodeID != "":
		rows, err = s.q.ListContainersByNodeId(ctx, opts.NodeID)
	case opts.ActiveOnly:
		rows, err = s.q.ListWorkloads(ctx)
	default:
		rows, err = s.q.ListContainers(ctx)
	}
	if err != nil {
		return nil, wrap("list containers", err)
	}
	out := make([]v1.Container, 0, len(rows))
	for _, row := range rows {
		out = append(out, *containerFromRow(row))
	}
	return out, nil
}

func (s *Store) ScheduleContainer(ctx context.Context, id string) (*v1.Container, error) {
	row, err := s.q.ScheduleContainer(ctx, id)
	if err != nil {
		return nil, wrap("schedule container", err)
	}
	return containerFromRow(row), nil
}

func (s *Store) MarkContainerRemoved(ctx context.Context, id string) (*v1.Container, error) {
	row, err := s.q.MarkContainerRemoved(ctx, id)
	if err != nil {
		return nil, wrap("mark container removed", err)
	}
	return containerFromRow(row), nil
}

func (s *Store) UpdateContainerRuntime(ctx context.Context, id string, u storage.RuntimeUpdate) (*v1.Container, error) {
	row, err := s.q.UpdateContainerRuntime(ctx, db.UpdateContainerRuntimeParams{
		ID:           id,
		ContainerdID: toText(u.ContainerdID),
		SnapshotKey:  toText(u.SnapshotKey),
		TaskPid:      toInt4(u.TaskPID),
		ImageDigest:  toText(u.ImageDigest),
		OverlayIp:    parseAddr(u.OverlayIP),
		Status:       db.ContainerStatus(u.Status),
		ExitCode:     toInt4(u.ExitCode),
		ErrorMessage: toText(u.ErrorMessage),
		RestartCount: toInt4(u.RestartCount),
		LastFailedAt: toTimestamptz(u.LastFailedAt),
		ScheduledAt:  toTimestamptz(u.ScheduledAt),
		StartedAt:    toTimestamptz(u.StartedAt),
		StoppedAt:    toTimestamptz(u.StoppedAt),
	})
	if err != nil {
		return nil, wrap("update container runtime", err)
	}
	return containerFromRow(row), nil
}

func (s *Store) NodeResourceUsage(ctx context.Context, nodeID string) (*storage.ResourceUsage, error) {
	count, err := s.q.CountRunningContainersByNodeId(ctx, nodeID)
	if err != nil {
		return nil, wrap("count node containers", err)
	}
	totals, err := s.q.SumContainerResourcesByNodeId(ctx, nodeID)
	if err != nil {
		return nil, wrap("sum node resources", err)
	}
	cpu, _ := totals.TotalCpu.Float64Value()
	return &storage.ResourceUsage{
		RunningCount:   int(count),
		TotalCPU:       cpu.Float64,
		TotalMemoryMiB: totals.TotalMemoryMib,
	}, nil
}

func (s *Store) HasActiveContainerWithImage(ctx context.Context, nodeID, image string) (bool, error) {
	ok, err := s.q.HasContainerWithImageOnNode(ctx, db.HasContainerWithImageOnNodeParams{NodeID: nodeID, Image: image})
	return ok, wrap("check image on node", err)
}

func (s *Store) AddContainerEvent(ctx context.Context, ev v1.ContainerEvent) error {
	id, err := newID("evt_")
	if err != nil {
		return err
	}
	err = s.q.CreateContainerEvent(ctx, db.CreateContainerEventParams{
		ID:          id,
		ContainerID: ev.ContainerID,
		NodeID:      ev.NodeID,
		Event:       db.ContainerEventType(ev.Event),
		Message:     toText(ev.Message),
	})
	return wrap("add container event", err)
}

func (s *Store) ListContainerEvents(ctx context.Context, containerID string) ([]v1.ContainerEvent, error) {
	rows, err := s.q.ListContainerEventsByContainerId(ctx, containerID)
	if err != nil {
		return nil, wrap("list container events", err)
	}
	out := make([]v1.ContainerEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, v1.ContainerEvent{
			ID:          row.ID,
			ContainerID: row.ContainerID,
			NodeID:      row.NodeID,
			Event:       v1.ContainerEventType(row.Event),
			Message:     textPtr(row.Message),
			Metadata:    unmarshalAnyMap(row.Metadata),
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *Store) SeedOverlayPool(ctx context.Context, nodeID string, ips []string) error {
	for _, ip := range ips {
		addr := parseAddr(ip)
		if addr == nil {
			return fmt.Errorf("seed overlay pool: invalid ip %q", ip)
		}
		if _, err := s.q.CreateOverlayIPPoolEntry(ctx, db.CreateOverlayIPPoolEntryParams{Ip: *addr, NodeID: nodeID}); err != nil {
			return wrap("seed overlay pool "+ip, err)
		}
	}
	return nil
}

func (s *Store) DeleteOverlayPool(ctx context.Context, nodeID string) error {
	return wrap("delete overlay pool", s.q.DeleteOverlayIPPoolByNodeId(ctx, nodeID))
}

func (s *Store) ListOverlayIPs(ctx context.Context, nodeID string, freeOnly bool) ([]v1.OverlayIP, error) {
	var (
		rows []db.OverlayIpPool
		err  error
	)
	if freeOnly {
		rows, err = s.q.ListFreeOverlayIPsByNodeId(ctx, nodeID)
	} else {
		rows, err = s.q.ListOverlayIPsByNodeId(ctx, nodeID)
	}
	if err != nil {
		return nil, wrap("list overlay ips", err)
	}
	out := make([]v1.OverlayIP, 0, len(rows))
	for _, row := range rows {
		out = append(out, v1.OverlayIP{
			IP:          row.Ip.Unmap().String(),
			NodeID:      row.NodeID,
			ContainerID: textPtr(row.ContainerID),
			AllocatedAt: timePtr(row.AllocatedAt),
		})
	}
	return out, nil
}

func (s *Store) CountFreeOverlayIPs(ctx context.Context, nodeID string) (int, error) {
	n, err := s.q.CountFreeOverlayIPsByNodeId(ctx, nodeID)
	return int(n), wrap("count free overlay ips", err)
}

func (s *Store) AllocateOverlayIP(ctx context.Context, nodeID, containerID string) (string, error) {
	row, err := s.q.AllocateFirstFreeOverlayIP(ctx, db.AllocateFirstFreeOverlayIPParams{
		NodeID:      nodeID,
		ContainerID: toTextNonEmpty(containerID),
	})
	if err != nil {
		return "", wrap("allocate overlay ip", err)
	}
	return row.Ip.Unmap().String(), nil
}

func (s *Store) ReleaseOverlayIP(ctx context.Context, ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("release overlay ip: %w", err)
	}
	return wrap("release overlay ip", s.q.ReleaseOverlayIP(ctx, addr))
}

func containerFromRow(row db.Container) *v1.Container {
	cpuShares := int32(1024)
	if row.CpuShares.Valid {
		cpuShares = row.CpuShares.Int32
	}
	var ports []v1.ExposedPort
	if len(row.ExposedPorts) > 0 {
		_ = json.Unmarshal(row.ExposedPorts, &ports)
	}

	return &v1.Container{
		ID:             row.ID,
		NodeID:         row.NodeID,
		ContainerdNs:   row.ContainerdNs,
		ContainerdID:   textPtr(row.ContainerdID),
		SnapshotKey:    textPtr(row.SnapshotKey),
		TaskPID:        int4Ptr(row.TaskPid),
		Image:          row.Image,
		ImageDigest:    textPtr(row.ImageDigest),
		Command:        row.Command,
		Entrypoint:     row.Entrypoint,
		Env:            unmarshalStringMap(row.Env),
		WorkingDir:     textPtr(row.WorkingDir),
		OverlayIP:      addrString(row.OverlayIp),
		ExposedPorts:   ports,
		CpuLimit:       numericPtr(row.CpuLimit),
		CpuShares:      cpuShares,
		MemoryLimitMiB: int4Ptr(row.MemoryLimitMib),
		Status:         v1.ContainerStatus(row.Status),
		ExitCode:       int4Ptr(row.ExitCode),
		ErrorMessage:   textPtr(row.ErrorMessage),
		RestartCount:   row.RestartCount,
		RestartPolicy:  v1.RestartPolicy(row.RestartPolicy),
		LastFailedAt:   timePtr(row.LastFailedAt),
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
		ScheduledAt:    timePtr(row.ScheduledAt),
		StartedAt:      timePtr(row.StartedAt),
		StoppedAt:      timePtr(row.StoppedAt),
	}
}
