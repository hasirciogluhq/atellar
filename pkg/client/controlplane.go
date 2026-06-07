package client

import (
	"fmt"
)

type ControlPlane struct {
	Address  string
	HTTPPort int
	GRPCPort int
}

func (cp ControlPlane) Validate() error {
	if cp.Address == "" {
		return fmt.Errorf("control plane address is required")
	}
	if cp.HTTPPort <= 0 {
		return fmt.Errorf("http port is required")
	}
	if cp.GRPCPort <= 0 {
		return fmt.Errorf("grpc port is required")
	}
	return nil
}

func (cp ControlPlane) HTTPBaseURL() string {
	return fmt.Sprintf("http://%s:%d", cp.Address, cp.HTTPPort)
}

func (cp ControlPlane) GRPCAddr() string {
	return fmt.Sprintf("%s:%d", cp.Address, cp.GRPCPort)
}
