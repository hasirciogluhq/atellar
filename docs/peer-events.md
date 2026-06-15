# Peer Events

When cluster state changes, the control plane pushes async `reconcile.trigger` RPCs to connected agents. Agents update overlay bridges and routes accordingly.

## Transport

- **Channel:** gRPC `Connect` stream → `ServerEnvelope.RpcCall`
- **Method:** `reconcile.trigger`
- **Mode:** `DELIVERY_MODE_ASYNC`
- **Payload:** JSON

## Node events

| Event | When | Excluded |
|-------|------|----------|
| `node.added` | Register completed | New node |
| `node.removed` | Evict | Evicted node |
| `node.updated` | Overlay IP/subnet changed | Updated node |

### `node.added` / `node.removed` payload

```json
{
  "event": "node.added",
  "node_id": "uuid",
  "name": "node-1",
  "overlay_ip": "10.0.0.1",
  "overlay_subnet": "10.0.0.0/24",
  "public_ip": "203.0.113.10",
  "private_ip": "10.0.0.5"
}
```

### `node.updated` payload

Previous values are included:

```json
{
  "event": "node.updated",
  "node_id": "uuid",
  "name": "node-1",
  "overlay_ip": "10.0.1.1",
  "overlay_subnet": "10.0.1.0/24",
  "previous_overlay_ip": "10.0.0.1",
  "previous_overlay_subnet": "10.0.0.0/24"
}
```

Triggered by: `PATCH /api/v1/nodes/:nodeId/overlay`

## Workload dispatch (target node only)

| Event | When | Target |
|-------|------|--------|
| `workload.dispatch` | Container scheduled to node | **Only** target node |
| `workload.removed` | Container delete requested | **Only** target node |

Payload:

```json
{
  "event": "workload.dispatch",
  "container_id": "ctr_...",
  "node_id": "node_..."
}
```

Agent triggers workload reconcile immediately (in addition to 15s poll).

## Container events (peer overlay)

| Event | When | Excluded |
|-------|------|----------|
| `container.scheduled` | Container created | Container's node |
| `container.started` | Status → `running` | Container's node |
| `container.stopped` | Status → `stopped` | Container's node |
| `container.terminated` | Status → `terminated` | Container's node |
| `container.updated` | Overlay IP updated at runtime | Container's node |

### Container payload

```json
{
  "event": "container.started",
  "container_id": "uuid",
  "node_id": "node-uuid",
  "overlay_ip": "10.0.0.50",
  "status": "running",
  "image": "nginx:alpine"
}
```

Peer nodes add routes to reach the container overlay IP.

## Subnet reclaim

Evicted node subnets remain in the DB for reclaim. On new node registration:

1. Reclaimable subnet is assigned first if available
2. Evicted node's overlay fields are cleared
3. `node.added` is sent to peers

On evict, `node.removed` is sent immediately (overlay still valid; peers remove routes).

## Agent side

`internal/agent/overlay/` — bridge, IP, and route reconcile (`linux` build tag).

`internal/agent/grpcclient/` forwards `reconcile.trigger` events to the overlay manager.

Details: [agent.md](agent.md). Cluster sync uses gRPC `GetClusterNetworkState`, not HTTP.

## Related code

- `internal/grpc/agentregistry/events.go` — event constants + push
- `internal/grpc/agentregistry/peer_notifier.go` — node + container notify
- `internal/controlplane/transport/http/routes/v1/container/container.routes.go` — container event wiring
- `internal/modules/nodes/application/usecases/update-node-overlay.useCase.go`
