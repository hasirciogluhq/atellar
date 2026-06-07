package server

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
)

func ListenAndServe(port string, deps Deps) error {
	grpcServer := grpc.NewServer()

	agentService := NewAgentService(deps)
	agentService.Register(grpcServer)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}

	return grpcServer.Serve(listener)
}
