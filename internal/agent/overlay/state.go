package overlay

import (
	"net"
	"sync"
)

type desiredState struct {
	mu         sync.RWMutex
	local      LocalNode
	peers      map[string]PeerNode
	containers map[string]RemoteContainer
}

func newDesiredState(local LocalNode) *desiredState {
	return &desiredState{
		local:      local,
		peers:      make(map[string]PeerNode),
		containers: make(map[string]RemoteContainer),
	}
}

func (s *desiredState) snapshot() (LocalNode, []PeerNode, []RemoteContainer) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]PeerNode, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	containers := make([]RemoteContainer, 0, len(s.containers))
	for _, item := range s.containers {
		containers = append(containers, item)
	}

	return s.local, peers, containers
}

func (s *desiredState) applyPeerEvent(event PeerEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch event.Event {
	case "node.added", "node.updated":
		if event.NodeID == "" || event.NodeID == s.local.NodeID {
			return
		}

		overlayIP := net.ParseIP(event.OverlayIP)
		_, overlaySubnet, err := net.ParseCIDR(event.OverlaySubnet)
		if overlayIP == nil || err != nil {
			return
		}

		if event.Event == "node.updated" && event.PreviousOverlaySubnet != "" {
			delete(s.peers, event.NodeID)
		}

		s.peers[event.NodeID] = PeerNode{
			NodeID:        event.NodeID,
			OverlayIP:     overlayIP,
			OverlaySubnet: overlaySubnet,
		}
	case "node.removed":
		delete(s.peers, event.NodeID)
		s.removeContainersOnNodeLocked(event.NodeID)
	case "container.scheduled", "container.started", "container.updated":
		s.upsertContainerLocked(event)
	case "container.stopped", "container.terminated":
		delete(s.containers, event.ContainerID)
	}
}

func (s *desiredState) upsertContainerLocked(event PeerEvent) {
	if event.ContainerID == "" || event.NodeID == s.local.NodeID {
		return
	}

	overlayIP := net.ParseIP(event.OverlayIP)
	if overlayIP == nil {
		return
	}

	peer, ok := s.peers[event.NodeID]
	if !ok || peer.OverlayIP == nil {
		return
	}

	s.containers[event.ContainerID] = RemoteContainer{
		ContainerID: event.ContainerID,
		NodeID:      event.NodeID,
		OverlayIP:   overlayIP,
		Via:         peer.OverlayIP,
	}
}

func (s *desiredState) removeContainersOnNodeLocked(nodeID string) {
	for id, item := range s.containers {
		if item.NodeID == nodeID {
			delete(s.containers, id)
		}
	}
}

func (s *desiredState) syncFromCluster(nodes []ClusterNode, containers []ClusterContainer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextPeers := make(map[string]PeerNode)
	for _, item := range nodes {
		if item.ID == s.local.NodeID {
			if item.OverlayIP != nil && item.OverlaySubnet != "" {
				_, subnet, err := net.ParseCIDR(item.OverlaySubnet)
				if err == nil {
					s.local.OverlayIP = item.OverlayIP
					s.local.OverlaySubnet = subnet
				}
			}
			continue
		}

		if item.Status == nodeStatusEvicted || item.Status == nodeStatusEvicting || item.Status == nodeStatusDown {
			continue
		}

		if item.OverlayIP == nil || item.OverlaySubnet == "" {
			continue
		}

		_, subnet, err := net.ParseCIDR(item.OverlaySubnet)
		if err != nil {
			continue
		}

		nextPeers[item.ID] = PeerNode{
			NodeID:        item.ID,
			OverlayIP:     item.OverlayIP,
			OverlaySubnet: subnet,
		}
	}
	s.peers = nextPeers

	nextContainers := make(map[string]RemoteContainer)
	for _, item := range containers {
		if item.NodeID == s.local.NodeID || item.OverlayIP == nil {
			continue
		}

		if !containerNeedsRoute(item.Status) {
			continue
		}

		peer, ok := s.peers[item.NodeID]
		if !ok || peer.OverlayIP == nil {
			continue
		}

		nextContainers[item.ID] = RemoteContainer{
			ContainerID: item.ID,
			NodeID:      item.NodeID,
			OverlayIP:   item.OverlayIP,
			Via:         peer.OverlayIP,
		}
	}
	s.containers = nextContainers
}

func (s *desiredState) buildRoutes() []RouteSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make([]RouteSpec, 0, len(s.peers)+len(s.containers))
	device := s.local.BridgeName

	for _, peer := range s.peers {
		if peer.OverlaySubnet == nil || peer.OverlayIP == nil {
			continue
		}

		if s.local.OverlaySubnet != nil && subnetsOverlap(s.local.OverlaySubnet, peer.OverlaySubnet) {
			continue
		}

		routes = append(routes, RouteSpec{
			Destination: peer.OverlaySubnet,
			Via:         peer.OverlayIP,
			Device:      device,
		})
	}

	for _, item := range s.containers {
		if item.OverlayIP == nil || item.Via == nil {
			continue
		}

		if s.local.OverlaySubnet != nil && s.local.OverlaySubnet.Contains(item.OverlayIP) {
			continue
		}

		routes = append(routes, RouteSpec{
			Destination: &net.IPNet{IP: item.OverlayIP, Mask: net.CIDRMask(32, 32)},
			Via:         item.Via,
			Device:      device,
		})
	}

	return routes
}

func containerNeedsRoute(status string) bool {
	switch status {
	case containerStatusScheduled, containerStatusPending, containerStatusPulling,
		containerStatusCreating, containerStatusRunning, containerStatusBackoff:
		return true
	default:
		return false
	}
}

func subnetsOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
