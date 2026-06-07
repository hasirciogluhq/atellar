package server

import (
	"context"
	"io"
	"log"
	"time"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	containerusecases "github.com/hasirciogluhq/atellar/internal/modules/containers/application/usecases"
	nodeusecases "github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentService struct {
	atellarv1.UnimplementedAgentServiceServer
	deps Deps
}

func NewAgentService(deps Deps) *AgentService {
	return &AgentService{deps: deps}
}

func (s *AgentService) Register(grpcServer *grpc.Server) {
	atellarv1.RegisterAgentServiceServer(grpcServer, s)
}

func (s *AgentService) Connect(stream grpc.BidiStreamingServer[atellarv1.AgentEnvelope, atellarv1.ServerEnvelope]) error {
	ctx, principal, err := authn.AuthenticateGRPC(stream.Context(), s.deps.NodeAuth)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%v", err)
	}

	node, err := authn.MustNodeFromContext(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%v", err)
	}

	_ = principal

	log.Printf("agent connected node_id=%s name=%s", node.ID, node.Name)

	send := func(envelope *atellarv1.ServerEnvelope) error {
		return stream.Send(envelope)
	}
	s.deps.AgentRegistry.Register(node.ID, send)
	defer s.deps.AgentRegistry.Unregister(node.ID)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch payload := msg.Payload.(type) {
		case *atellarv1.AgentEnvelope_Heartbeat:
			if err := s.deps.Nodes.UpdateNodeHeartbeat(stream.Context(), node.ID); err != nil {
				return status.Errorf(codes.Internal, "heartbeat failed: %v", err)
			}

			if err := stream.Send(&atellarv1.ServerEnvelope{
				CorrelationId: msg.CorrelationId,
				Payload: &atellarv1.ServerEnvelope_HeartbeatAck{
					HeartbeatAck: &atellarv1.HeartbeatAck{
						TimestampUnix: time.Now().Unix(),
					},
				},
			}); err != nil {
				return err
			}
		case *atellarv1.AgentEnvelope_Ingest:
			if err := stream.Send(&atellarv1.ServerEnvelope{
				CorrelationId: msg.CorrelationId,
				Payload: &atellarv1.ServerEnvelope_IngestAck{
					IngestAck: &atellarv1.IngestAck{
						Accepted: true,
						Message:  payload.Ingest.GetEventType(),
					},
				},
			}); err != nil {
				return err
			}
		case *atellarv1.AgentEnvelope_RpcResult:
			continue
		default:
			continue
		}
	}
}

func (s *AgentService) RenewNodeAPIKey(ctx context.Context, _ *atellarv1.RenewNodeAPIKeyRequest) (*atellarv1.RenewNodeAPIKeyResponse, error) {
	credential, err := authn.ParseGRPCMetadataFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	if _, err := s.deps.NodeAuth.Authenticate(ctx, credential); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	useCase := nodeusecases.NewRenewNodeAPIKeyUseCase(s.deps.Nodes)
	result, err := useCase.Execute(ctx, credential.Value)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	return &atellarv1.RenewNodeAPIKeyResponse{
		NodeApiKey:    result.NodeAPIKey,
		ExpiresAtUnix: result.APIKeyExpiresAt.Unix(),
	}, nil
}

func (s *AgentService) GetClusterNetworkState(ctx context.Context, _ *atellarv1.GetClusterNetworkStateRequest) (*atellarv1.GetClusterNetworkStateResponse, error) {
	if _, _, err := authn.AuthenticateGRPC(ctx, s.deps.NodeAuth); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	listNodes := nodeusecases.NewListNodesUseCase(s.deps.Nodes)
	nodes, err := listNodes.Execute(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list nodes: %v", err)
	}

	listContainers := containerusecases.NewListContainersUseCase(s.deps.Containers)
	containers, err := listContainers.Execute(ctx, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	resp := &atellarv1.GetClusterNetworkStateResponse{
		Nodes:      make([]*atellarv1.ClusterNode, 0, len(nodes)),
		Containers: make([]*atellarv1.ClusterContainer, 0, len(containers)),
	}

	for _, item := range nodes {
		resp.Nodes = append(resp.Nodes, &atellarv1.ClusterNode{
			Id:            item.ID,
			OverlayIp:     item.OverlayIP.String(),
			OverlaySubnet: item.OverlaySubnet,
			Status:        string(item.Status),
		})
	}

	for _, item := range containers {
		resp.Containers = append(resp.Containers, &atellarv1.ClusterContainer{
			Id:        item.ID,
			NodeId:    item.NodeID,
			OverlayIp: item.OverlayIP.String(),
			Status:    string(item.Status),
		})
	}

	return resp, nil
}
