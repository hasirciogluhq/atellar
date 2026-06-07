package agentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	"github.com/hasirciogluhq/atellar/internal/pkg/agentconfig"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
	"github.com/hasirciogluhq/atellar/internal/pkg/nodetoken"
	"github.com/hasirciogluhq/atellar/internal/pkg/overlaynet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type networkReconciler interface {
	HandlePeerEvent(event overlaynet.PeerEvent)
}

type Session struct {
	cfg        *agentconfig.Config
	configPath string
	conn       *grpc.ClientConn
	client     atellarv1.AgentServiceClient
	network    networkReconciler
}

func NewSession(cfg *agentconfig.Config, configPath string, network networkReconciler) (*Session, error) {
	conn, err := grpc.NewClient(
		cfg.ResolveGrpcAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	return &Session{
		cfg:        cfg,
		configPath: configPath,
		conn:       conn,
		client:     atellarv1.NewAgentServiceClient(conn),
		network:    network,
	}, nil
}

func (s *Session) Close() error {
	return s.conn.Close()
}

func (s *Session) SetNetworkReconciler(network networkReconciler) {
	s.network = network
}

func (s *Session) Run(ctx context.Context, heartbeatEvery time.Duration) error {
	streamCtx := authn.OutgoingContext(ctx, authn.Credential{
		Type:  authn.CredentialTypeNodeAPIKey,
		Value: s.cfg.NodeAPIKey,
	})

	stream, err := s.client.Connect(streamCtx)
	if err != nil {
		return fmt.Errorf("connect stream: %w", err)
	}

	recvDone := make(chan error, 1)
	go func() {
		recvDone <- s.recvLoop(stream)
	}()

	renewTicker := time.NewTicker(time.Hour)
	defer renewTicker.Stop()

	heartbeatTicker := time.NewTicker(heartbeatEvery)
	defer heartbeatTicker.Stop()

	if err := s.sendHeartbeat(stream); err != nil {
		log.Printf("initial heartbeat failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvDone:
			return err
		case <-heartbeatTicker.C:
			if err := s.sendHeartbeat(stream); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		case <-renewTicker.C:
			if nodetoken.ShouldRenew(s.cfg.APIKeyExpiresAt) {
				if err := s.renewAPIKey(ctx); err != nil {
					log.Printf("api key renew failed: %v", err)
				}
			}
		}
	}
}

func (s *Session) recvLoop(stream grpc.BidiStreamingClient[atellarv1.AgentEnvelope, atellarv1.ServerEnvelope]) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch payload := msg.Payload.(type) {
		case *atellarv1.ServerEnvelope_RpcCall:
			s.handleRpcCall(payload.RpcCall)
		default:
			continue
		}
	}
}

func (s *Session) handleRpcCall(call *atellarv1.RpcCall) {
	if call == nil {
		return
	}

	switch call.GetMethod() {
	case "reconcile.trigger":
		var payload overlaynet.PeerEvent
		if err := json.Unmarshal(call.GetPayload(), &payload); err != nil {
			log.Printf("reconcile.trigger invalid payload: %v", err)
			return
		}

		log.Printf(
			"reconcile.trigger event=%s node_id=%s container_id=%s overlay_ip=%s overlay_subnet=%s",
			payload.Event,
			payload.NodeID,
			payload.ContainerID,
			payload.OverlayIP,
			payload.OverlaySubnet,
		)

		if s.network != nil {
			s.network.HandlePeerEvent(payload)
		}
	default:
		log.Printf("unhandled rpc call method=%s", call.GetMethod())
	}
}

func (s *Session) sendHeartbeat(stream grpc.BidiStreamingClient[atellarv1.AgentEnvelope, atellarv1.ServerEnvelope]) error {
	return stream.Send(&atellarv1.AgentEnvelope{
		CorrelationId: fmt.Sprintf("hb-%d", time.Now().UnixNano()),
		Payload: &atellarv1.AgentEnvelope_Heartbeat{
			Heartbeat: &atellarv1.Heartbeat{
				TimestampUnix: time.Now().Unix(),
			},
		},
	})
}

func (s *Session) renewAPIKey(ctx context.Context) error {
	renewCtx := authn.OutgoingContext(ctx, authn.Credential{
		Type:  authn.CredentialTypeNodeAPIKey,
		Value: s.cfg.NodeAPIKey,
	})

	resp, err := s.client.RenewNodeAPIKey(renewCtx, &atellarv1.RenewNodeAPIKeyRequest{})
	if err != nil {
		return err
	}

	s.cfg.NodeAPIKey = resp.GetNodeApiKey()
	s.cfg.APIKeyExpiresAt = time.Unix(resp.GetExpiresAtUnix(), 0)

	if err := agentconfig.Save(s.configPath, *s.cfg); err != nil {
		return err
	}

	log.Printf("node api key renewed expires_at=%s", s.cfg.APIKeyExpiresAt.Format(time.RFC3339))
	return nil
}
