package postgres

import (
	"context"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/postgres/db"
)

const defaultContainerdSock = "/run/containerd/containerd.sock"

func (s *Store) GetNode(ctx context.Context, id string) (*v1.Node, error) {
	row, err := s.q.GetNodeById(ctx, id)
	if err != nil {
		return nil, wrap("get node", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) GetNodeByName(ctx context.Context, name string) (*v1.Node, error) {
	row, err := s.q.GetNodeByName(ctx, name)
	if err != nil {
		return nil, wrap("get node by name", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) GetNodeByAPIKeyHash(ctx context.Context, hash string) (*v1.Node, error) {
	row, err := s.q.GetNodeByTokenHash(ctx, toTextNonEmpty(hash))
	if err != nil {
		return nil, wrap("get node by api key", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) ListNodes(ctx context.Context) ([]v1.Node, error) {
	rows, err := s.q.ListNodes(ctx)
	if err != nil {
		return nil, wrap("list nodes", err)
	}
	return nodesFromRows(rows), nil
}

func (s *Store) ListSchedulableNodes(ctx context.Context) ([]v1.Node, error) {
	rows, err := s.q.ListSchedulableNodes(ctx)
	if err != nil {
		return nil, wrap("list schedulable nodes", err)
	}
	return nodesFromRows(rows), nil
}

func (s *Store) CreateNode(ctx context.Context, params storage.CreateNodeParams) (*v1.Node, error) {
	id, err := newID(v1.NodeIDPrefix)
	if err != nil {
		return nil, err
	}
	name := params.Name
	if name == "" {
		name = id
	}
	sock := params.ContainerdSock
	if sock == "" {
		sock = defaultContainerdSock
	}

	row, err := s.q.CreateNode(ctx, db.CreateNodeParams{
		ID:             id,
		Name:           name,
		PublicIp:       parseAddr(params.PublicIP),
		PrivateIp:      parseAddr(params.PrivateIP),
		AgentVersion:   toText(params.AgentVersion),
		ContainerdSock: sock,
		Status:         db.NodeStatusPending,
	})
	if err != nil {
		return nil, wrap("create node", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) HeartbeatNode(ctx context.Context, id string) (*v1.Node, error) {
	row, err := s.q.UpdateNodeHeartbeat(ctx, id)
	if err != nil {
		return nil, wrap("heartbeat node", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) SetNodeStatus(ctx context.Context, id string, status v1.NodeStatus) (*v1.Node, error) {
	row, err := s.q.SetNodeStatus(ctx, db.SetNodeStatusParams{ID: id, Status: db.NodeStatus(status)})
	if err != nil {
		return nil, wrap("set node status", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) MarkStaleNodesNotReady(ctx context.Context, staleBefore time.Time) ([]v1.Node, error) {
	rows, err := s.q.MarkStaleNodesNotReady(ctx, pgtype.Timestamptz{Time: staleBefore, Valid: true})
	if err != nil {
		return nil, wrap("mark stale nodes", err)
	}
	return nodesFromRows(rows), nil
}

func (s *Store) EvictNode(ctx context.Context, id string) (*v1.Node, error) {
	row, err := s.q.EvictNode(ctx, id)
	if err != nil {
		return nil, wrap("evict node", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) UpdateNodeHardware(ctx context.Context, id string, hw v1.NodeHardware) (*v1.Node, error) {
	row, err := s.q.UpdateNodeHardware(ctx, db.UpdateNodeHardwareParams{
		ID:             id,
		CpuCores:       pgtype.Int4{Int32: hw.CpuCores, Valid: true},
		MemoryTotalMib: pgtype.Int4{Int32: hw.MemoryTotalMiB, Valid: true},
		DiskTotalGib:   pgtype.Int4{Int32: hw.DiskTotalGiB, Valid: true},
		Hostname:       toText(&hw.Hostname),
		Os:             toText(&hw.OS),
		Arch:           toText(&hw.Arch),
		KernelVersion:  toText(&hw.KernelVersion),
	})
	if err != nil {
		return nil, wrap("update node hardware", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) SetNodeAPIKey(ctx context.Context, id, hash string, expiresAt time.Time) error {
	_, err := s.q.UpdateNodeToken(ctx, db.UpdateNodeTokenParams{
		ID:             id,
		TokenHash:      toTextNonEmpty(hash),
		TokenExpiresAt: toTimestamptz(&expiresAt),
	})
	return wrap("set node api key", err)
}

func (s *Store) LockOverlayAllocation(ctx context.Context) error {
	return wrap("lock overlay allocation", s.q.LockOverlayAllocation(ctx))
}

func (s *Store) SetNodeOverlay(ctx context.Context, id, overlayIP, subnetCIDR string, status v1.NodeStatus) (*v1.Node, error) {
	prefix, err := netip.ParsePrefix(subnetCIDR)
	if err != nil {
		return nil, err
	}
	row, err := s.q.UpdateNodeOverlayNetwork(ctx, db.UpdateNodeOverlayNetworkParams{
		ID:            id,
		OverlayIp:     parseAddr(overlayIP),
		OverlaySubnet: &prefix,
		Status:        db.NodeStatus(status),
	})
	if err != nil {
		return nil, wrap("set node overlay", err)
	}
	return nodeFromRow(row), nil
}

func (s *Store) ClearNodeOverlay(ctx context.Context, id string) error {
	return wrap("clear node overlay", s.q.ClearNodeOverlayNetwork(ctx, id))
}

func (s *Store) ListActiveOverlaySubnets(ctx context.Context) ([]string, error) {
	subnets, err := s.q.ListActiveNodeOverlaySubnets(ctx)
	return subnets, wrap("list active overlay subnets", err)
}

func (s *Store) ListReclaimableOverlaySubnets(ctx context.Context) ([]storage.ReclaimableSubnet, error) {
	rows, err := s.q.ListReclaimableOverlayNetworks(ctx)
	if err != nil {
		return nil, wrap("list reclaimable overlay subnets", err)
	}
	out := make([]storage.ReclaimableSubnet, 0, len(rows))
	for _, row := range rows {
		out = append(out, storage.ReclaimableSubnet{NodeID: row.ID, SubnetCIDR: row.OverlaySubnet})
	}
	return out, nil
}

func (s *Store) CreateJoinToken(ctx context.Context, hash string, singleUse bool, expiresAt *time.Time) (*v1.JoinToken, error) {
	id, err := newID("jtok_")
	if err != nil {
		return nil, err
	}
	row, err := s.q.CreateJoinToken(ctx, db.CreateJoinTokenParams{
		ID:        id,
		TokenHash: hash,
		SingleUse: singleUse,
		ExpiresAt: toTimestamptz(expiresAt),
	})
	if err != nil {
		return nil, wrap("create join token", err)
	}
	return joinTokenFromRow(row), nil
}

func (s *Store) GetJoinTokenByHash(ctx context.Context, hash string) (*v1.JoinToken, error) {
	row, err := s.q.GetJoinTokenByHash(ctx, hash)
	if err != nil {
		return nil, wrap("get join token", err)
	}
	return joinTokenFromRow(row), nil
}

func (s *Store) ListJoinTokens(ctx context.Context) ([]v1.JoinToken, error) {
	rows, err := s.q.ListJoinTokens(ctx)
	if err != nil {
		return nil, wrap("list join tokens", err)
	}
	out := make([]v1.JoinToken, 0, len(rows))
	for _, row := range rows {
		out = append(out, *joinTokenFromRow(row))
	}
	return out, nil
}

func (s *Store) MarkJoinTokenUsed(ctx context.Context, id, nodeID string) error {
	n, err := s.q.MarkJoinTokenUsed(ctx, db.MarkJoinTokenUsedParams{ID: id, UsedBy: toTextNonEmpty(nodeID)})
	if err != nil {
		return wrap("mark join token used", err)
	}
	if n == 0 {
		return storage.ErrConflict
	}
	return nil
}

func nodesFromRows(rows []db.Node) []v1.Node {
	out := make([]v1.Node, 0, len(rows))
	for _, row := range rows {
		out = append(out, *nodeFromRow(row))
	}
	return out
}

func nodeFromRow(row db.Node) *v1.Node {
	return &v1.Node{
		ID:                 row.ID,
		Name:               row.Name,
		PublicIP:           addrString(row.PublicIp),
		PrivateIP:          addrString(row.PrivateIp),
		OverlayIP:          addrString(row.OverlayIp),
		OverlaySubnet:      prefixString(row.OverlaySubnet),
		Status:             v1.NodeStatus(row.Status),
		LastHeartbeat:      timePtr(row.LastHeartbeat),
		AgentVersion:       textPtr(row.AgentVersion),
		ContainerdSock:     row.ContainerdSock,
		CpuCores:           int4Ptr(row.CpuCores),
		MemoryTotalMiB:     int4Ptr(row.MemoryTotalMib),
		DiskTotalGiB:       int4Ptr(row.DiskTotalGib),
		Hostname:           textPtr(row.Hostname),
		OS:                 textPtr(row.Os),
		Arch:               textPtr(row.Arch),
		KernelVersion:      textPtr(row.KernelVersion),
		HardwareReportedAt: timePtr(row.HardwareReportedAt),
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}
}

func joinTokenFromRow(row db.NodeJoinToken) *v1.JoinToken {
	return &v1.JoinToken{
		ID:        row.ID,
		SingleUse: row.SingleUse,
		UsedAt:    timePtr(row.UsedAt),
		UsedBy:    textPtr(row.UsedBy),
		ExpiresAt: timePtr(row.ExpiresAt),
		CreatedAt: row.CreatedAt.Time,
	}
}
