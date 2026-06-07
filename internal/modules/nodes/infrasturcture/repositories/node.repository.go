package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"

	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	joinToken "github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/join-token"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

	return r.ParseRestNode(row), nil
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

	return r.ParseRestNode(row), nil
}

func (r *NodeRepository) CreateNode(ctx context.Context, name string, publicIP, privateIP net.IP) (*node.NodeEntity, error) {
	nodeID, err := generatePrefixedID("node_")
	if err != nil {
		return nil, err
	}

	if name == "" {
		name = nodeID
	}

	row, err := r.queries.CreateNode(ctx, db_generated.CreateNodeParams{
		ID:        nodeID,
		Name:      name,
		PublicIp:  ipToText(publicIP),
		PrivateIp: ipToText(privateIP),
		Status:    db_generated.NodeStatusPending,
	})
	if err != nil {
		fmt.Println("Error creating node: ", err)
		return nil, err
	}

	return r.ParseRestNode(row), nil
}

func (r *NodeRepository) CreateJoinToken(ctx context.Context, expiresAt *time.Time) (*joinToken.JoinTokenEntity, error) {
	tokenID, err := generatePrefixedID("jtok_")
	if err != nil {
		return nil, err
	}

	tokenValue, err := generateRandomHex(32)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.CreateJoinToken(ctx, db_generated.CreateJoinTokenParams{
		ID:        tokenID,
		Token:     tokenValue,
		ExpiresAt: timeToTimestamptz(expiresAt),
	})
	if err != nil {
		fmt.Println("Error creating join token: ", err)
		return nil, err
	}

	return r.parseJoinToken(row), nil
}

func (r *NodeRepository) GetJoinToken(ctx context.Context, token string) (*joinToken.JoinTokenEntity, error) {
	row, err := r.queries.GetJoinTokenByToken(ctx, token)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}

		fmt.Println("Error getting join token: ", err)

		return nil, err
	}

	return r.parseJoinToken(row), nil
}

func (r *NodeRepository) ListJoinTokens(ctx context.Context) ([]joinToken.JoinTokenEntity, error) {
	rows, err := r.queries.ListJoinTokens(ctx)
	if err != nil {
		fmt.Println("Error listing join tokens: ", err)
		return nil, err
	}

	joinTokens := make([]joinToken.JoinTokenEntity, 0, len(rows))
	for _, row := range rows {
		joinTokens = append(joinTokens, *r.parseJoinToken(row))
	}

	return joinTokens, nil
}

func (r *NodeRepository) ParseRestNode(row db_generated.Node) *node.NodeEntity {
	publicIP := net.ParseIP(row.PublicIp.String)
	if publicIP == nil {
		publicIP = nil
	}
	privateIP := net.ParseIP(row.PrivateIp.String)
	if privateIP == nil {
		privateIP = nil
	}

	var lastHeartbeat *time.Time
	if row.LastHeartbeat.Valid {
		lastHeartbeat = &row.LastHeartbeat.Time
	}

	return &node.NodeEntity{
		ID:            row.ID,
		Name:          row.Name,
		PublicIP:      publicIP,
		PrivateIP:     privateIP,
		Status:        node.NodeStatus(row.Status),
		LastHeartbeat: lastHeartbeat,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}

func (r *NodeRepository) parseJoinToken(row db_generated.NodeJoinToken) *joinToken.JoinTokenEntity {
	var expiresAt *time.Time
	if row.ExpiresAt.Valid {
		expiresAt = &row.ExpiresAt.Time
	}

	return &joinToken.JoinTokenEntity{
		ID:        row.ID,
		Token:     row.Token,
		ExpiresAt: expiresAt,
		CreatedAt: row.CreatedAt.Time,
	}
}

func ipToText(ip net.IP) pgtype.Text {
	if ip == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{String: ip.String(), Valid: true}
}

func timeToTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func generatePrefixedID(prefix string) (string, error) {
	randomPart, err := generateRandomHex(8)
	if err != nil {
		return "", err
	}

	return prefix + randomPart, nil
}

func generateRandomHex(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
