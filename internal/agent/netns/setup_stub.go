//go:build !linux

package netns

import "errors"

type Config struct {
	ContainerID string
	OverlayIP   string
	BridgeName  string
	GatewayIP   string
}

func Setup(Config) error {
	return errors.New("netns setup only supported on linux")
}

func Teardown(string) {}

func NetnsPath(containerID string) string {
	return containerID
}
