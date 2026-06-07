# Agent

Binary: `atellar-agent` (`cmd/agent`)

Node-side process that connects to the control plane. **Dumb design**: reads config only, does not make orchestration decisions.

## How it works

1. Load `/etc/atellar/agent.json`
2. Resolve gRPC address (`grpc_addr` or control plane host + `:9090`)
3. Open `Connect` bidi stream with `Bearer <node_api_key>`
4. Send heartbeats (default 5s)
5. Handle `reconcile.trigger` RPCs from the server
6. Check API key expiry hourly; auto-renew 7 days before expiry

## Config file

Path: `/etc/atellar/agent.json`

```json
{
  "control_plane_url": "http://localhost:8080",
  "grpc_addr": "localhost:9090",
  "node_id": "uuid",
  "node_name": "node-1",
  "overlay_ip": "10.0.0.1",
  "overlay_subnet": "10.0.0.0/24",
  "node_api_key": "hex-key",
  "api_key_expires_at": "2026-06-07T12:00:00Z",
  "containerd_sock": "/run/containerd/containerd.sock",
  "heartbeat_interval": "5s",
  "bridge_name": "atellar0",
  "reconcile_interval": "30s"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `control_plane_url` | yes | HTTP API base |
| `node_id` | yes | Assigned after register |
| `node_api_key` | yes | gRPC auth |
| `api_key_expires_at` | yes | Renew threshold |
| `overlay_ip` | no | Set after register |
| `overlay_subnet` | no | Set after register |
| `grpc_addr` | no | Defaults to hostname:9090 |
| `heartbeat_interval` | no | Default `5s` |
| `bridge_name` | no | Overlay bridge (default `atellar0`) |
| `reconcile_interval` | no | Network reconcile period (default `30s`) |

The agent does **not** use environment variables.

## Overlay network (Linux)

On startup, `overlaynet.Manager` runs:

1. **Bridge** — creates and brings UP `bridge_name` (default `atellar0`)
2. **Local IP** — assigns `overlay_ip/overlay_subnet` to the bridge
3. **Peer routes** — `via <peer_overlay_ip> dev <bridge>` for other node subnets
4. **Container routes** — `/32 via <node_overlay_ip>` for remote container overlay IPs
5. **Continuous reconcile** — on every `reconcile_interval` and every `reconcile.trigger` event
6. **Cluster sync** — periodically fetches node/container lists from the API (drift correction)

Uses `ip link`, `ip addr`, `ip route` (`CAP_NET_ADMIN` required).

On macOS/dev: stub mode logs only, no real network changes.

## gRPC stream

Proto: `api/proto/atellar/v1/agent.proto`

| Direction | Message | Description |
|-----------|---------|-------------|
| Agent → Server | `Heartbeat` | Periodic liveness |
| Agent → Server | `Ingest` | Event ingest (MVP: ack only) |
| Server → Agent | `HeartbeatAck` | Heartbeat response |
| Server → Agent | `RpcCall` | `reconcile.trigger` peer events |

## Peer reconcile

On `reconcile.trigger`, the agent updates desired network state and reconciles:

- `node.added` / `node.removed` / `node.updated`
- `container.scheduled` / `container.started` / `container.stopped` / `container.terminated` / `container.updated`

## systemd

The `install` command creates an `atellar-agent.service` unit:

- Config: `/etc/atellar/agent.json`
- Binary: `/usr/local/bin/atellar-agent`

## Related code

- `internal/agent/agent.go` — entry point
- `internal/grpc/agentclient/session.go` — stream, heartbeat, renew, RPC handler
- `internal/pkg/overlaynet/` — bridge, IP, route reconcile
- `internal/pkg/agentconfig/` — config load/save
