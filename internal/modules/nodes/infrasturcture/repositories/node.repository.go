package postgres

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	joinToken "github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/join-token"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
	"github.com/hasirciogluhq/atellar/internal/pkg/nodetoken"
	"github.com/hasirciogluhq/atellar/internal/pkg/pgutil"
	"github.com/hasirciogluhq/atellar/internal/pkg/tokenhash"
	"github.com/jackc/pgx/v5"
)

type NodeRepository struct {
	queries *db_generated.Queries
}

func NewNodeRepository(queries *db_generated.Queries) *NodeRepository {
	return &NodeRepository{queries: queries}
}

func (r *NodeRepository) UpdateNodeHeartbeat(ctx context.Context, nodeID string) error {
	err := r.queries.UpdateNodeHeartbeat(ctx, nodeID)
	if err != nil {
		log.Printf("Error updating node heartbeat: %v", err)
		return err
	}
	return nil
}

func (r *NodeRepository) GetNodeById(ctx context.Context, nodeID string) (*node.NodeEntity, error) {
	row, err := r.queries.GetNodeById(ctx, nodeID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error getting node by id: ", err)
		return nil, err
	}

	return parseNode(row), nil
}

func (r *NodeRepository) GetNodeByName(ctx context.Context, nodeName string) (*node.NodeEntity, error) {
	row, err := r.queries.GetNodeByName(ctx, nodeName)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error getting node by name: ", err)
		return nil, err
	}

	return parseNode(row), nil
}

func (r *NodeRepository) ListNodes(ctx context.Context) ([]node.NodeEntity, error) {
	rows, err := r.queries.ListNodes(ctx)
	if err != nil {
		fmt.Println("Error listing nodes: ", err)
		return nil, err
	}

	nodes := make([]node.NodeEntity, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, *parseNode(row))
	}

	return nodes, nil
}

func (r *NodeRepository) CreateNode(ctx context.Context, input ports.CreateNodeInput) (*node.NodeEntity, error) {
	nodeID, err := pgutil.GeneratePrefixedID("node_")
	if err != nil {
		return nil, err
	}

	name := input.Name
	if name == "" {
		name = nodeID
	}

	containerdSock := input.ContainerdSock
	if containerdSock == "" {
		containerdSock = "/run/containerd/containerd.sock"
	}

	row, err := r.queries.CreateNode(ctx, db_generated.CreateNodeParams{
		ID:             nodeID,
		Name:           name,
		PublicIp:       pgutil.NetIPToAddr(input.PublicIP),
		PrivateIp:      pgutil.NetIPToAddr(input.PrivateIP),
		AgentVersion:   pgutil.StringToText(input.AgentVersion),
		ContainerdSock: containerdSock,
		Status:         db_generated.NodeStatusPending,
	})
	if err != nil {
		fmt.Println("Error creating node: ", err)
		return nil, err
	}

	return parseNode(row), nil
}

func (r *NodeRepository) IssueNodeAPIKey(ctx context.Context, nodeID string) (*node.NodeAPIKeyResult, error) {
	issued, err := nodetoken.Issue()
	if err != nil {
		return nil, err
	}

	_, err = r.queries.UpdateNodeToken(ctx, db_generated.UpdateNodeTokenParams{
		ID:             nodeID,
		TokenHash:      pgutil.OptionalStringToText(tokenhash.Hash(issued.PlainToken)),
		TokenExpiresAt: pgutil.TimeToTimestamptz(&issued.ExpiresAt),
	})
	if err != nil {
		fmt.Println("Error issuing node api key: ", err)
		return nil, err
	}

	return &node.NodeAPIKeyResult{
		NodeAPIKey:      issued.PlainToken,
		APIKeyExpiresAt: issued.ExpiresAt,
	}, nil
}

func (r *NodeRepository) AuthenticateNodeByAPIKey(ctx context.Context, plainAPIKey string) (*node.NodeEntity, error) {
	row, err := r.queries.GetNodeByTokenHash(ctx, pgutil.OptionalStringToText(tokenhash.Hash(plainAPIKey)))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error authenticating node api key: ", err)
		return nil, err
	}

	return parseNode(row), nil
}

func (r *NodeRepository) RenewNodeAPIKey(ctx context.Context, plainAPIKey string) (*node.NodeAPIKeyResult, error) {
	authenticated, err := r.AuthenticateNodeByAPIKey(ctx, plainAPIKey)
	if err != nil {
		return nil, err
	}

	if authenticated == nil {
		return nil, fmt.Errorf("invalid node api key")
	}

	return r.IssueNodeAPIKey(ctx, authenticated.ID)
}

func (r *NodeRepository) CreateJoinToken(ctx context.Context, expiresAt *time.Time, singleUse bool) (*joinToken.JoinTokenCreateResult, error) {
	tokenID, err := pgutil.GeneratePrefixedID("jtok_")
	if err != nil {
		return nil, err
	}

	plainToken, err := pgutil.GenerateRandomHex(32)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.CreateJoinToken(ctx, db_generated.CreateJoinTokenParams{
		ID:        tokenID,
		TokenHash: tokenhash.Hash(plainToken),
		SingleUse: singleUse,
		ExpiresAt: pgutil.TimeToTimestamptz(expiresAt),
	})
	if err != nil {
		fmt.Println("Error creating join token: ", err)
		return nil, err
	}

	entity := parseJoinToken(row)
	return &joinToken.JoinTokenCreateResult{
		JoinTokenEntity: *entity,
		Token:           plainToken,
	}, nil
}

func (r *NodeRepository) GetJoinToken(ctx context.Context, plainToken string) (*joinToken.JoinTokenEntity, error) {
	row, err := r.queries.GetJoinTokenByHash(ctx, tokenhash.Hash(plainToken))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error getting join token: ", err)
		return nil, err
	}

	return parseJoinToken(row), nil
}

func (r *NodeRepository) ListJoinTokens(ctx context.Context) ([]joinToken.JoinTokenEntity, error) {
	rows, err := r.queries.ListJoinTokens(ctx)
	if err != nil {
		fmt.Println("Error listing join tokens: ", err)
		return nil, err
	}

	joinTokens := make([]joinToken.JoinTokenEntity, 0, len(rows))
	for _, row := range rows {
		joinTokens = append(joinTokens, *parseJoinToken(row))
	}

	return joinTokens, nil
}

func (r *NodeRepository) MarkJoinTokenUsed(ctx context.Context, tokenID, nodeID string) error {
	err := r.queries.MarkJoinTokenUsed(ctx, db_generated.MarkJoinTokenUsedParams{
		ID:     tokenID,
		UsedBy: pgutil.OptionalStringToText(nodeID),
	})
	if err != nil {
		fmt.Println("Error marking join token used: ", err)
		return err
	}

	return nil
}

func (r *NodeRepository) ListActiveNodeOverlaySubnets(ctx context.Context) ([]string, error) {
	subnets, err := r.queries.ListActiveNodeOverlaySubnets(ctx)
	if err != nil {
		fmt.Println("Error listing active node overlay subnets: ", err)
		return nil, err
	}

	return subnets, nil
}

func (r *NodeRepository) ListReclaimableOverlayNetworks(ctx context.Context) ([]ports.ReclaimableOverlayNetwork, error) {
	rows, err := r.queries.ListReclaimableOverlayNetworks(ctx)
	if err != nil {
		fmt.Println("Error listing reclaimable overlay networks: ", err)
		return nil, err
	}

	result := make([]ports.ReclaimableOverlayNetwork, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.ReclaimableOverlayNetwork{
			NodeID:     row.ID,
			SubnetCIDR: row.OverlaySubnet,
		})
	}

	return result, nil
}

func (r *NodeRepository) ClearNodeOverlayNetwork(ctx context.Context, nodeID string) error {
	if err := r.queries.ClearNodeOverlayNetwork(ctx, nodeID); err != nil {
		fmt.Println("Error clearing node overlay network: ", err)
		return err
	}

	return nil
}

func (r *NodeRepository) EvictNode(ctx context.Context, nodeID string) (*node.NodeEntity, error) {
	row, err := r.queries.EvictNode(ctx, nodeID)
	if err != nil {
		fmt.Println("Error evicting node: ", err)
		return nil, err
	}

	return parseNode(row), nil
}

func (r *NodeRepository) UpdateNodeOverlayNetwork(ctx context.Context, nodeID string, overlayIP net.IP, subnetCIDR string, status node.NodeStatus) (*node.NodeEntity, error) {
	prefix, err := pgutil.CIDRToPrefix(subnetCIDR)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.UpdateNodeOverlayNetwork(ctx, db_generated.UpdateNodeOverlayNetworkParams{
		ID:            nodeID,
		OverlayIp:     pgutil.NetIPToAddr(overlayIP),
		OverlaySubnet: prefix,
		Status:        db_generated.NodeStatus(status),
	})
	if err != nil {
		fmt.Println("Error updating node overlay network: ", err)
		return nil, err
	}

	return parseNode(row), nil
}

func parseNode(row db_generated.Node) *node.NodeEntity {
	return &node.NodeEntity{
		ID:             row.ID,
		Name:           row.Name,
		PublicIP:       pgutil.AddrToNetIP(row.PublicIp),
		PrivateIP:      pgutil.AddrToNetIP(row.PrivateIp),
		OverlayIP:      pgutil.AddrToNetIP(row.OverlayIp),
		OverlaySubnet:  pgutil.PrefixToString(row.OverlaySubnet),
		Status:         node.NodeStatus(row.Status),
		LastHeartbeat:  pgutil.TimestamptzToTime(row.LastHeartbeat),
		AgentVersion:   pgutil.TextToString(row.AgentVersion),
		ContainerdSock: row.ContainerdSock,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

func parseJoinToken(row db_generated.NodeJoinToken) *joinToken.JoinTokenEntity {
	return &joinToken.JoinTokenEntity{
		ID:        row.ID,
		SingleUse: row.SingleUse,
		UsedAt:    pgutil.TimestamptzToTime(row.UsedAt),
		UsedBy:    pgutil.TextToString(row.UsedBy),
		ExpiresAt: pgutil.TimestamptzToTime(row.ExpiresAt),
		CreatedAt: row.CreatedAt.Time,
	}
}
