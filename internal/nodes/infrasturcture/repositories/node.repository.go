package postgres

import (
	"context"
	"fmt"
	"log"
	"net"

	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	"github.com/hasirciogluhq/atellar/internal/nodes/domain/node"
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

func (r *NodeRepository) ParseRestNode(row db_generated.Node) *node.NodeEntity {
	publicIP := net.ParseIP(row.PublicIp.String)
	if publicIP == nil {
		publicIP = nil
	}
	privateIP := net.ParseIP(row.PrivateIp.String)
	if privateIP == nil {
		privateIP = nil
	}

	return &node.NodeEntity{
		ID:            row.ID,
		Name:          row.Name,
		PublicIP:      publicIP,
		PrivateIP:     privateIP,
		Status:        node.NodeStatus(row.Status),
		LastHeartbeat: &row.LastHeartbeat.Time,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}
