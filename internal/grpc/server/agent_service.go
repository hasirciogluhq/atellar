package server

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentService struct {
	atellarv1.UnimplementedAgentServiceServer
	infra *shared.Infrastructure
}

func NewAgentService(infra *shared.Infrastructure) *AgentService {
	return &AgentService{infra: infra}
}

func (s *AgentService) Register(grpcServer *grpc.Server) {
	atellarv1.RegisterAgentServiceServer(grpcServer, s)
}

func (s *AgentService) Connect(stream grpc.BidiStreamingServer[atellarv1.AgentEnvelope, atellarv1.ServerEnvelope]) error {
	ctx, principal, err := authn.AuthenticateGRPC(stream.Context(), s.infra.NodeAuth)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%v", err)
	}

	node, err := authn.MustNodeFromContext(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%v", err)
	}

	log.Printf("agent connected node_id=%s name=%s", node.ID, node.Name)
	_ = principal

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
			if err := s.infra.Repositories.Nodes.UpdateNodeHeartbeat(stream.Context(), node.ID); err != nil {
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

	if _, err := s.infra.NodeAuth.Authenticate(ctx, credential); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	useCase := usecases.NewRenewNodeAPIKeyUseCase(s.infra.Repositories.Nodes)
	result, err := useCase.Execute(ctx, credential.Value)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	return &atellarv1.RenewNodeAPIKeyResponse{
		NodeApiKey:    result.NodeAPIKey,
		ExpiresAtUnix: result.APIKeyExpiresAt.Unix(),
	}, nil
}
