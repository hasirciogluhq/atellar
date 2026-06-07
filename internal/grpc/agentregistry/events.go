package agentregistry

import (
	"encoding/json"
	"log"

	atellarv1 "github.com/hasirciogluhq/atellar/internal/grpc/gen/atellar/v1"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

const (
	PeerEventNodeAdded   = "node.added"
	PeerEventNodeRemoved = "node.removed"
	PeerEventNodeUpdated = "node.updated"

	PeerEventContainerScheduled  = "container.scheduled"
	PeerEventContainerStarted    = "container.started"
	PeerEventContainerStopped    = "container.stopped"
	PeerEventContainerTerminated = "container.terminated"
	PeerEventContainerUpdated    = "container.updated"
)

type nodePeerEventPayload struct {
	Event                 string `json:"event"`
	NodeID                string `json:"node_id"`
	Name                  string `json:"name"`
	OverlayIP             string `json:"overlay_ip,omitempty"`
	OverlaySubnet         string `json:"overlay_subnet,omitempty"`
	PreviousOverlayIP     string `json:"previous_overlay_ip,omitempty"`
	PreviousOverlaySubnet string `json:"previous_overlay_subnet,omitempty"`
	PublicIP              string `json:"public_ip,omitempty"`
	PrivateIP             string `json:"private_ip,omitempty"`
}

type containerPeerEventPayload struct {
	Event       string `json:"event"`
	ContainerID string `json:"container_id"`
	NodeID      string `json:"node_id"`
	OverlayIP   string `json:"overlay_ip,omitempty"`
	Status      string `json:"status,omitempty"`
	Image       string `json:"image,omitempty"`
}

func (r *Registry) sendReconcileEvent(correlationID string, payload []byte, excludeNodeID string) int {
	envelope := &atellarv1.ServerEnvelope{
		CorrelationId: correlationID,
		Payload: &atellarv1.ServerEnvelope_RpcCall{
			RpcCall: &atellarv1.RpcCall{
				Method:  reconcileTriggerMethod,
				Payload: payload,
				Mode:    atellarv1.DeliveryMode_DELIVERY_MODE_ASYNC,
			},
		},
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	notified := 0
	for nodeID, connection := range r.connections {
		if excludeNodeID != "" && nodeID == excludeNodeID {
			continue
		}

		if err := connection.Send(envelope); err != nil {
			log.Printf("reconcile event send to %s failed: %v", nodeID, err)
			continue
		}

		notified++
	}

	return notified
}

func (r *Registry) NotifyNodeEvent(event string, target node.NodeEntity, excludeNodeID string, previousOverlayIP, previousOverlaySubnet string) int {
	payload, err := json.Marshal(nodePeerEventPayload{
		Event:                 event,
		NodeID:                target.ID,
		Name:                  target.Name,
		OverlayIP:             target.OverlayIP.String(),
		OverlaySubnet:         target.OverlaySubnet,
		PreviousOverlayIP:     previousOverlayIP,
		PreviousOverlaySubnet: previousOverlaySubnet,
		PublicIP:              target.PublicIP.String(),
		PrivateIP:             target.PrivateIP.String(),
	})
	if err != nil {
		log.Printf("marshal %s payload failed: %v", event, err)
		return 0
	}

	return r.sendReconcileEvent(event+"-"+target.ID, payload, excludeNodeID)
}

func (r *Registry) NotifyNodeAdded(newNode node.NodeEntity) int {
	return r.NotifyNodeEvent(PeerEventNodeAdded, newNode, newNode.ID, "", "")
}

func (r *Registry) NotifyNodeRemoved(removedNode node.NodeEntity) int {
	return r.NotifyNodeEvent(PeerEventNodeRemoved, removedNode, removedNode.ID, "", "")
}

func (r *Registry) NotifyNodeUpdated(updatedNode node.NodeEntity, previousOverlayIP, previousOverlaySubnet string) int {
	return r.NotifyNodeEvent(
		PeerEventNodeUpdated,
		updatedNode,
		updatedNode.ID,
		previousOverlayIP,
		previousOverlaySubnet,
	)
}

func (r *Registry) NotifyContainerEvent(event string, target container.Entity) int {
	payload, err := json.Marshal(containerPeerEventPayload{
		Event:       event,
		ContainerID: target.ID,
		NodeID:      target.NodeID,
		OverlayIP:   target.OverlayIP.String(),
		Status:      string(target.Status),
		Image:       target.Image,
	})
	if err != nil {
		log.Printf("marshal %s payload failed: %v", event, err)
		return 0
	}

	return r.sendReconcileEvent(event+"-"+target.ID, payload, target.NodeID)
}
