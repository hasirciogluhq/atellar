package v1

import (
	"errors"
	"net"
	"strings"
)

// NodeIDPrefix marks generated node IDs; anything else is treated as a name.
const NodeIDPrefix = "node_"

// IsNodeID reports whether ref is a node ID rather than a node name.
func IsNodeID(ref string) bool {
	return strings.HasPrefix(ref, NodeIDPrefix)
}

func (r RegisterNodeRequest) Validate() error {
	if strings.TrimSpace(r.JoinToken) == "" {
		return errors.New("join token is required")
	}
	if r.PublicIP != "" && net.ParseIP(r.PublicIP) == nil {
		return errors.New("public_ip is not a valid IP")
	}
	if r.PrivateIP != "" && net.ParseIP(r.PrivateIP) == nil {
		return errors.New("private_ip is not a valid IP")
	}
	return nil
}

func (h NodeHardware) Validate() error {
	if h.CpuCores <= 0 || h.MemoryTotalMiB <= 0 {
		return errors.New("cpu_cores and memory_total_mib are required")
	}
	return nil
}

func (r CreateContainerRequest) Validate() error {
	if strings.TrimSpace(r.Image) == "" {
		return errors.New("image is required")
	}
	switch r.RestartPolicy {
	case "", RestartPolicyNo, RestartPolicyAlways, RestartPolicyOnFailure:
	default:
		return errors.New("restart_policy must be one of no, always, on-failure")
	}
	if r.CpuLimit != nil && *r.CpuLimit < 0 {
		return errors.New("cpu_limit must not be negative")
	}
	if r.MemoryLimitMiB != nil && *r.MemoryLimitMiB < 0 {
		return errors.New("memory_limit_mib must not be negative")
	}
	return nil
}

func (r ContainerRuntimeReport) Validate() error {
	if r.Status == "" {
		return errors.New("status is required")
	}
	if r.OverlayIP != "" && net.ParseIP(r.OverlayIP) == nil {
		return errors.New("overlay_ip is not a valid IP")
	}
	return nil
}
