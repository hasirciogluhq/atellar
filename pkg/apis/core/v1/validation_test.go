package v1

import "testing"

func ptr[T any](v T) *T { return &v }

func TestIsNodeID(t *testing.T) {
	for ref, want := range map[string]bool{"node_abc": true, "node-1": false, "": false, "web": false} {
		if got := IsNodeID(ref); got != want {
			t.Errorf("IsNodeID(%q) = %v, want %v", ref, got, want)
		}
	}
}

func TestRegisterNodeRequestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  RegisterNodeRequest
		ok   bool
	}{
		{"token only", RegisterNodeRequest{JoinToken: "t"}, true},
		{"full", RegisterNodeRequest{JoinToken: "t", PublicIP: "1.2.3.4", PrivateIP: "10.0.0.1"}, true},
		{"ipv6", RegisterNodeRequest{JoinToken: "t", PrivateIP: "fd00::1"}, true},
		{"no token", RegisterNodeRequest{}, false},
		{"blank token", RegisterNodeRequest{JoinToken: "  "}, false},
		{"bad public", RegisterNodeRequest{JoinToken: "t", PublicIP: "x"}, false},
		{"bad private", RegisterNodeRequest{JoinToken: "t", PrivateIP: "10.0.0"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestNodeHardwareValidate(t *testing.T) {
	cases := []struct {
		hw NodeHardware
		ok bool
	}{
		{NodeHardware{CpuCores: 2, MemoryTotalMiB: 1024}, true},
		{NodeHardware{CpuCores: 0, MemoryTotalMiB: 1024}, false},
		{NodeHardware{CpuCores: 2}, false},
		{NodeHardware{CpuCores: -1, MemoryTotalMiB: -1}, false},
	}
	for _, tc := range cases {
		if err := tc.hw.Validate(); (err == nil) != tc.ok {
			t.Errorf("%+v: Validate() = %v, want ok=%v", tc.hw, err, tc.ok)
		}
	}
}

func TestCreateContainerRequestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  CreateContainerRequest
		ok   bool
	}{
		{"image", CreateContainerRequest{Image: "nginx"}, true},
		{"no policy", CreateContainerRequest{Image: "nginx", RestartPolicy: ""}, true},
		{"policy no", CreateContainerRequest{Image: "nginx", RestartPolicy: RestartPolicyNo}, true},
		{"policy always", CreateContainerRequest{Image: "nginx", RestartPolicy: RestartPolicyAlways}, true},
		{"policy on-failure", CreateContainerRequest{Image: "nginx", RestartPolicy: RestartPolicyOnFailure}, true},
		{"limits", CreateContainerRequest{Image: "nginx", CpuLimit: ptr(0.5), MemoryLimitMiB: ptr[int32](128)}, true},
		{"zero limits", CreateContainerRequest{Image: "nginx", CpuLimit: ptr(0.0), MemoryLimitMiB: ptr[int32](0)}, true},
		{"no image", CreateContainerRequest{}, false},
		{"blank image", CreateContainerRequest{Image: " "}, false},
		{"bad policy", CreateContainerRequest{Image: "nginx", RestartPolicy: "sometimes"}, false},
		{"negative cpu", CreateContainerRequest{Image: "nginx", CpuLimit: ptr(-1.0)}, false},
		{"negative memory", CreateContainerRequest{Image: "nginx", MemoryLimitMiB: ptr[int32](-1)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestContainerRuntimeReportValidate(t *testing.T) {
	cases := []struct {
		name string
		r    ContainerRuntimeReport
		ok   bool
	}{
		{"status", ContainerRuntimeReport{Status: ContainerStatusRunning}, true},
		{"with ip", ContainerRuntimeReport{Status: ContainerStatusRunning, OverlayIP: "10.0.1.5"}, true},
		{"no status", ContainerRuntimeReport{}, false},
		{"bad ip", ContainerRuntimeReport{Status: ContainerStatusRunning, OverlayIP: "nope"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.r.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}
