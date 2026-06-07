package server

import (
	"fmt"
	"net"

	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"google.golang.org/grpc"
)

func ListenAndServe(port string, infra *shared.Infrastructure) error {
	grpcServer := grpc.NewServer()

	agentService := NewAgentService(infra)
	agentService.Register(grpcServer)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

	return grpcServer.Serve(listener)
}
