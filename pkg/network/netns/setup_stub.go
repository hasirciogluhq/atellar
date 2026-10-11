//go:build !linux

package netns

import "errors"

type Config struct {
	ContainerID       string
	OverlayIP         string
	NodeOverlayIP     string
	NodeOverlaySubnet string
	BridgeName        string
	// MTU of both veth ends; it must match the overlay MTU (default 1450).
	MTU int
}

func Setup(Config) error {
	return errors.New("netns setup only supported on linux")
}

func Teardown(string) {}

func NetnsPath(containerID string) string {
	return containerID
}
