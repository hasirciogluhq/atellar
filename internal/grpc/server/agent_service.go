package server

import (
	"context"
	"io"
	"log"
	"net"
	"time"

	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	containerusecases "github.com/hasirciogluhq/atellar/internal/modules/containers/application/usecases"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
	nodeusecases "github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
	nodeports "github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
	"github.com/hasirciogluhq/atellar/internal/platform/authz"
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

func (s *AgentService) authenticateAndAuthorize(ctx context.Context, action authz.Action) (context.Context, error) {
	authCtx, _, err := authn.AuthenticateGRPC(ctx, s.deps.NodeAuth)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}
	if err := s.authorize(authCtx, action); err != nil {
		return nil, err
	}
	return authCtx, nil
}

func (s *AgentService) authorize(ctx context.Context, action authz.Action) error {
	authorizer := s.deps.Authz
	if authorizer == nil {
		authorizer = authz.New(nil)
	}
	if err := authorizer.Assert(ctx, action); err != nil {
		return status.Errorf(codes.PermissionDenied, "%v", err)
	}
	return nil
}

func (s *AgentService) Connect(stream grpc.BidiStreamingServer[atellarv1.AgentEnvelope, atellarv1.ServerEnvelope]) error {
	ctx, err := s.authenticateAndAuthorize(stream.Context(), authz.ActionAgentConnect)
	if err != nil {
		return err
	}

	node, err := authn.MustNodeFromContext(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "%v", err)
	}

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
			if err := s.authorize(ctx, authz.ActionAgentHeartbeat); err != nil {
				return err
			}
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

	principal, err := s.deps.NodeAuth.Authenticate(ctx, credential)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}
	authCtx := authn.WithPrincipal(ctx, principal)
	if err := s.authorize(authCtx, authz.ActionNodeRenewAPIKey); err != nil {
		return nil, err
	}

	useCase := nodeusecases.NewRenewNodeAPIKeyUseCase(s.deps.Nodes)
	result, err := useCase.Execute(authCtx, credential.Value)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	return &atellarv1.RenewNodeAPIKeyResponse{
		NodeApiKey:    result.NodeAPIKey,
		ExpiresAtUnix: result.APIKeyExpiresAt.Unix(),
	}, nil
}

func (s *AgentService) GetClusterNetworkState(ctx context.Context, _ *atellarv1.GetClusterNetworkStateRequest) (*atellarv1.GetClusterNetworkStateResponse, error) {
	authCtx, err := s.authenticateAndAuthorize(ctx, authz.ActionAgentReadClusterNetwork)
	if err != nil {
		return nil, err
	}

	listNodes := nodeusecases.NewListNodesUseCase(s.deps.Nodes)
	nodes, err := listNodes.Execute(authCtx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list nodes: %v", err)
	}

	listContainers := containerusecases.NewListContainersUseCase(s.deps.Containers)
	containers, err := listContainers.Execute(authCtx, "")
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

func (s *AgentService) GetNodeWorkloads(ctx context.Context, _ *atellarv1.GetNodeWorkloadsRequest) (*atellarv1.GetNodeWorkloadsResponse, error) {
	authCtx, err := s.authenticateAndAuthorize(ctx, authz.ActionAgentReadWorkloads)
	if err != nil {
		return nil, err
	}
	nodeID, err := authn.ResolveNodeIDFromContext(authCtx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	useCase := containerusecases.NewGetNodeWorkloadsUseCase(s.deps.Containers)
	workloads, err := useCase.Execute(authCtx, nodeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list workloads: %v", err)
	}

	resp := &atellarv1.GetNodeWorkloadsResponse{Workloads: make([]*atellarv1.Workload, 0, len(workloads))}
	for _, w := range workloads {
		resp.Workloads = append(resp.Workloads, workloadToProto(w))
	}
	return resp, nil
}

func (s *AgentService) ReportContainerRuntime(ctx context.Context, req *atellarv1.ReportContainerRuntimeRequest) (*atellarv1.ReportContainerRuntimeResponse, error) {
	authCtx, err := s.authenticateAndAuthorize(ctx, authz.ActionAgentReportRuntime)
	if err != nil {
		return nil, err
	}
	nodeID, err := authn.ResolveNodeIDFromContext(authCtx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	runtimeInput := ports.UpdateContainerRuntimeInput{
		ContainerdID: optionalString(req.GetContainerdId()),
		SnapshotKey:  optionalString(req.GetSnapshotKey()),
		TaskPID:      optionalInt32(req.GetTaskPid()),
		ImageDigest:  optionalString(req.GetImageDigest()),
		OverlayIP:    parseOverlayIP(req.GetOverlayIp()),
		Status:       container.Status(req.GetStatus()),
		ExitCode:     optionalInt32(req.GetExitCode()),
		ErrorMessage: optionalString(req.GetErrorMessage()),
		RestartCount: optionalInt32(req.GetRestartCount()),
	}
	if req.GetLastFailedAtUnix() > 0 {
		t := time.Unix(req.GetLastFailedAtUnix(), 0)
		runtimeInput.LastFailedAt = &t
	}

	useCase := containerusecases.NewReportContainerRuntimeUseCase(s.deps.Containers, s.deps.ContainerPeerNotifier)
	updated, err := useCase.Execute(authCtx, containerusecases.ReportContainerRuntimeInput{
		NodeID:      nodeID,
		ContainerID: req.GetContainerId(),
		Runtime:     runtimeInput,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &atellarv1.ReportContainerRuntimeResponse{OverlayIp: updated.OverlayIP.String()}, nil
}

func (s *AgentService) AllocateContainerOverlayIP(ctx context.Context, req *atellarv1.AllocateContainerOverlayIPRequest) (*atellarv1.AllocateContainerOverlayIPResponse, error) {
	authCtx, err := s.authenticateAndAuthorize(ctx, authz.ActionAgentAllocateOverlayIP)
	if err != nil {
		return nil, err
	}
	nodeID, err := authn.ResolveNodeIDFromContext(authCtx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	useCase := containerusecases.NewAllocateContainerOverlayIPUseCase(s.deps.Containers)
	ip, err := useCase.Execute(authCtx, nodeID, req.GetContainerId())
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
	}

	if s.deps.ContainerPeerNotifier != nil {
		found, _ := s.deps.Containers.GetContainerById(authCtx, req.GetContainerId())
		if found != nil {
			_ = s.deps.ContainerPeerNotifier.NotifyContainerEvent(authCtx, agentregistry.PeerEventContainerUpdated, *found)
		}
	}

	return &atellarv1.AllocateContainerOverlayIPResponse{OverlayIp: ip.String()}, nil
}

func (s *AgentService) ReportNodeHardware(ctx context.Context, req *atellarv1.ReportNodeHardwareRequest) (*atellarv1.ReportNodeHardwareResponse, error) {
	authCtx, err := s.authenticateAndAuthorize(ctx, authz.ActionAgentReportHardware)
	if err != nil {
		return nil, err
	}
	nodeID, err := authn.ResolveNodeIDFromContext(authCtx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "%v", err)
	}

	useCase := nodeusecases.NewUpdateNodeHardwareUseCase(s.deps.Nodes)
	_, err = useCase.Execute(authCtx, nodeID, nodeports.UpdateNodeHardwareInput{
		CpuCores:       req.GetCpuCores(),
		MemoryTotalMiB: req.GetMemoryTotalMib(),
		DiskTotalGiB:   req.GetDiskTotalGib(),
		Hostname:       req.GetHostname(),
		OS:             req.GetOs(),
		Arch:           req.GetArch(),
		KernelVersion:  req.GetKernelVersion(),
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &atellarv1.ReportNodeHardwareResponse{}, nil
}

func workloadToProto(w container.Entity) *atellarv1.Workload {
	out := &atellarv1.Workload{
		Id:            w.ID,
		Image:         w.Image,
		Command:       w.Command,
		Entrypoint:    w.Entrypoint,
		Env:           w.Env,
		ContainerdNs:  w.ContainerdNs,
		Status:        string(w.Status),
		RestartPolicy: string(w.RestartPolicy),
		OverlayIp:     w.OverlayIP.String(),
		RestartCount:  w.RestartCount,
		CpuShares:     w.CpuShares,
		ErrorMessage:  derefString(w.ErrorMessage),
	}
	if w.WorkingDir != nil {
		out.WorkingDir = *w.WorkingDir
	}
	if w.ImageDigest != nil {
		out.ImageDigest = *w.ImageDigest
	}
	if w.CpuLimit != nil {
		out.CpuLimit = *w.CpuLimit
	}
	if w.MemoryLimitMiB != nil {
		out.MemoryLimitMib = *w.MemoryLimitMiB
	}
	if w.LastFailedAt != nil {
		out.LastFailedAtUnix = w.LastFailedAt.Unix()
	}
	return out
}

func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func optionalInt32(v int32) *int32 {
	if v == 0 {
		return nil
	}
	return &v
}

func parseOverlayIP(v string) net.IP {
	if v == "" {
		return nil
	}
	return net.ParseIP(v)
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
