package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	containerevent "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container-event"
	overlayippool "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/overlay-ip-pool"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
	"github.com/hasirciogluhq/atellar/internal/platform/pgutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ContainerRepository struct {
	queries *db_generated.Queries
}

func NewContainerRepository(queries *db_generated.Queries) *ContainerRepository {
	return &ContainerRepository{queries: queries}
}

func (r *ContainerRepository) CreateContainer(ctx context.Context, input ports.CreateContainerInput) (*container.Entity, error) {
	containerID, err := pgutil.GeneratePrefixedID("ctr_")
	if err != nil {
		return nil, err
	}

	envBytes, err := pgutil.MarshalJSONMap(input.Env)
	if err != nil {
		return nil, err
	}

	restartPolicy := string(input.RestartPolicy)
	if restartPolicy == "" {
		restartPolicy = string(container.RestartPolicyNo)
	}

	cpuShares := int32(1024)
	if input.CpuShares != nil {
		cpuShares = *input.CpuShares
	}

	containerdNs := input.ContainerdNs
	if containerdNs == "" {
		containerdNs = "atellar"
	}

	row, err := r.queries.CreateContainer(ctx, db_generated.CreateContainerParams{
		ID:             containerID,
		NodeID:         input.NodeID,
		Image:          input.Image,
		Command:        input.Command,
		Entrypoint:     input.Entrypoint,
		Env:            envBytes,
		WorkingDir:     pgutil.StringToText(input.WorkingDir),
		CpuLimit:       pgutil.Float64ToNumeric(input.CpuLimit),
		CpuShares:      pgtype.Int4{Int32: cpuShares, Valid: true},
		MemoryLimitMib: pgutil.Int32PtrToInt4(input.MemoryLimitMiB),
		RestartPolicy:  restartPolicy,
		ContainerdNs:   containerdNs,
	})
	if err != nil {
		fmt.Println("Error creating container: ", err)
		return nil, err
	}

	return parseContainer(row), nil
}

func (r *ContainerRepository) GetContainerById(ctx context.Context, containerID string) (*container.Entity, error) {
	row, err := r.queries.GetContainerById(ctx, containerID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error getting container by id: ", err)
		return nil, err
	}

	return parseContainer(row), nil
}

func (r *ContainerRepository) ListContainers(ctx context.Context) ([]container.Entity, error) {
	rows, err := r.queries.ListContainers(ctx)
	if err != nil {
		fmt.Println("Error listing containers: ", err)
		return nil, err
	}

	return parseContainers(rows), nil
}

func (r *ContainerRepository) ListContainersByNodeId(ctx context.Context, nodeID string) ([]container.Entity, error) {
	rows, err := r.queries.ListContainersByNodeId(ctx, nodeID)
	if err != nil {
		fmt.Println("Error listing containers by node id: ", err)
		return nil, err
	}

	return parseContainers(rows), nil
}

func (r *ContainerRepository) UpdateContainerStatus(ctx context.Context, containerID string, status container.Status) (*container.Entity, error) {
	row, err := r.queries.UpdateContainerStatus(ctx, db_generated.UpdateContainerStatusParams{
		ID:     containerID,
		Status: db_generated.ContainerStatus(status),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error updating container status: ", err)
		return nil, err
	}

	return parseContainer(row), nil
}

func (r *ContainerRepository) UpdateContainerRuntime(ctx context.Context, containerID string, input ports.UpdateContainerRuntimeInput) (*container.Entity, error) {
	restartCount := int32(0)
	if input.RestartCount != nil {
		restartCount = *input.RestartCount
	}

	row, err := r.queries.UpdateContainerRuntime(ctx, db_generated.UpdateContainerRuntimeParams{
		ID:           containerID,
		ContainerdID: pgutil.StringToText(input.ContainerdID),
		SnapshotKey:  pgutil.StringToText(input.SnapshotKey),
		TaskPid:      pgutil.Int32PtrToInt4(input.TaskPID),
		ImageDigest:  pgutil.StringToText(input.ImageDigest),
		OverlayIp:    pgutil.NetIPToAddr(input.OverlayIP),
		Status:       db_generated.ContainerStatus(input.Status),
		ExitCode:     pgutil.Int32PtrToInt4(input.ExitCode),
		ErrorMessage: pgutil.StringToText(input.ErrorMessage),
		RestartCount: restartCount,
		ScheduledAt:  pgutil.TimeToTimestamptz(input.ScheduledAt),
		StartedAt:    pgutil.TimeToTimestamptz(input.StartedAt),
		StoppedAt:    pgutil.TimeToTimestamptz(input.StoppedAt),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error updating container runtime: ", err)
		return nil, err
	}

	return parseContainer(row), nil
}

func (r *ContainerRepository) CreateContainerEvent(ctx context.Context, input ports.CreateContainerEventInput) (*containerevent.Entity, error) {
	eventID, err := pgutil.GeneratePrefixedID("cevt_")
	if err != nil {
		return nil, err
	}

	metadata, err := pgutil.MarshalJSONRaw(input.Metadata)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.CreateContainerEvent(ctx, db_generated.CreateContainerEventParams{
		ID:          eventID,
		ContainerID: input.ContainerID,
		NodeID:      input.NodeID,
		Event:       db_generated.ContainerEventType(input.Event),
		Message:     pgutil.StringToText(input.Message),
		Metadata:    metadata,
	})
	if err != nil {
		fmt.Println("Error creating container event: ", err)
		return nil, err
	}

	return parseContainerEvent(row), nil
}

func (r *ContainerRepository) ListContainerEventsByContainerId(ctx context.Context, containerID string) ([]containerevent.Entity, error) {
	rows, err := r.queries.ListContainerEventsByContainerId(ctx, containerID)
	if err != nil {
		fmt.Println("Error listing container events: ", err)
		return nil, err
	}

	events := make([]containerevent.Entity, 0, len(rows))
	for _, row := range rows {
		events = append(events, *parseContainerEvent(row))
	}

	return events, nil
}

func (r *ContainerRepository) CreateOverlayIPPoolEntry(ctx context.Context, ip net.IP, nodeID string) (*overlayippool.Entity, error) {
	addr := pgutil.NetIPToAddr(ip)
	if addr == nil {
		return nil, fmt.Errorf("invalid overlay ip")
	}

	row, err := r.queries.CreateOverlayIPPoolEntry(ctx, db_generated.CreateOverlayIPPoolEntryParams{
		Ip:     *addr,
		NodeID: nodeID,
	})
	if err != nil {
		fmt.Println("Error creating overlay ip pool entry: ", err)
		return nil, err
	}

	return parseOverlayIPPool(row), nil
}

func (r *ContainerRepository) DeleteOverlayIPPoolByNodeId(ctx context.Context, nodeID string) error {
	if err := r.queries.DeleteOverlayIPPoolByNodeId(ctx, nodeID); err != nil {
		fmt.Println("Error deleting overlay ip pool by node id: ", err)
		return err
	}

	return nil
}

func (r *ContainerRepository) ListFreeOverlayIPsByNodeId(ctx context.Context, nodeID string) ([]overlayippool.Entity, error) {
	rows, err := r.queries.ListFreeOverlayIPsByNodeId(ctx, nodeID)
	if err != nil {
		fmt.Println("Error listing free overlay ips: ", err)
		return nil, err
	}

	return parseOverlayIPPools(rows), nil
}

func (r *ContainerRepository) ListOverlayIPsByNodeId(ctx context.Context, nodeID string) ([]overlayippool.Entity, error) {
	rows, err := r.queries.ListOverlayIPsByNodeId(ctx, nodeID)
	if err != nil {
		fmt.Println("Error listing overlay ips: ", err)
		return nil, err
	}

	return parseOverlayIPPools(rows), nil
}

func (r *ContainerRepository) AllocateOverlayIP(ctx context.Context, ip net.IP, containerID string) (*overlayippool.Entity, error) {
	addr := pgutil.NetIPToAddr(ip)
	if addr == nil {
		return nil, fmt.Errorf("invalid overlay ip")
	}

	row, err := r.queries.AllocateOverlayIP(ctx, db_generated.AllocateOverlayIPParams{
		Ip:          *addr,
		ContainerID: pgutil.OptionalStringToText(containerID),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error allocating overlay ip: ", err)
		return nil, err
	}

	return parseOverlayIPPool(row), nil
}

func (r *ContainerRepository) ReleaseOverlayIP(ctx context.Context, ip net.IP) error {
	addr := pgutil.NetIPToAddr(ip)
	if addr == nil {
		return fmt.Errorf("invalid overlay ip")
	}

	err := r.queries.ReleaseOverlayIP(ctx, *addr)
	if err != nil {
		log.Printf("Error releasing overlay ip: %v", err)
		return err
	}

	return nil
}

func parseContainers(rows []db_generated.Container) []container.Entity {
	containers := make([]container.Entity, 0, len(rows))
	for _, row := range rows {
		containers = append(containers, *parseContainer(row))
	}

	return containers
}

func parseContainer(row db_generated.Container) *container.Entity {
	cpuShares := int32(1024)
	if row.CpuShares.Valid {
		cpuShares = row.CpuShares.Int32
	}

	return &container.Entity{
		ID:             row.ID,
		NodeID:         row.NodeID,
		ContainerdNs:   row.ContainerdNs,
		ContainerdID:   pgutil.TextToString(row.ContainerdID),
		SnapshotKey:    pgutil.TextToString(row.SnapshotKey),
		TaskPID:        pgutil.Int4ToInt32Ptr(row.TaskPid),
		Image:          row.Image,
		ImageDigest:    pgutil.TextToString(row.ImageDigest),
		Command:        row.Command,
		Entrypoint:     row.Entrypoint,
		Env:            pgutil.UnmarshalJSONMap(row.Env),
		WorkingDir:     pgutil.TextToString(row.WorkingDir),
		OverlayIP:      pgutil.AddrToNetIP(row.OverlayIp),
		ExposedPorts:   parseExposedPorts(row.ExposedPorts),
		CpuLimit:       pgutil.NumericToFloat64(row.CpuLimit),
		CpuShares:      cpuShares,
		MemoryLimitMiB: pgutil.Int4ToInt32Ptr(row.MemoryLimitMib),
		Status:         container.Status(row.Status),
		ExitCode:       pgutil.Int4ToInt32Ptr(row.ExitCode),
		ErrorMessage:   pgutil.TextToString(row.ErrorMessage),
		RestartCount:   row.RestartCount,
		RestartPolicy:  container.RestartPolicy(row.RestartPolicy),
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
		ScheduledAt:    pgutil.TimestamptzToTime(row.ScheduledAt),
		StartedAt:      pgutil.TimestamptzToTime(row.StartedAt),
		StoppedAt:      pgutil.TimestamptzToTime(row.StoppedAt),
	}
}

func parseExposedPorts(data []byte) []container.ExposedPort {
	if len(data) == 0 {
		return nil
	}

	var ports []container.ExposedPort
	if err := json.Unmarshal(data, &ports); err != nil {
		return nil
	}

	return ports
}

func parseContainerEvent(row db_generated.ContainerEvent) *containerevent.Entity {
	return &containerevent.Entity{
		ID:          row.ID,
		ContainerID: row.ContainerID,
		NodeID:      row.NodeID,
		Event:       containerevent.EventType(row.Event),
		Message:     pgutil.TextToString(row.Message),
		Metadata:    pgutil.UnmarshalJSONRaw(row.Metadata),
		CreatedAt:   row.CreatedAt.Time,
	}
}

func parseOverlayIPPools(rows []db_generated.OverlayIpPool) []overlayippool.Entity {
	items := make([]overlayippool.Entity, 0, len(rows))
	for _, row := range rows {
		items = append(items, *parseOverlayIPPool(row))
	}

	return items
}

func parseOverlayIPPool(row db_generated.OverlayIpPool) *overlayippool.Entity {
	return &overlayippool.Entity{
		IP:          row.Ip.AsSlice(),
		NodeID:      row.NodeID,
		ContainerID: pgutil.TextToString(row.ContainerID),
		AllocatedAt: pgutil.TimestamptzToTime(row.AllocatedAt),
	}
}
