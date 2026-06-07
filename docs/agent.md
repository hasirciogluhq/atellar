# Agent

Binary: `atellar-agent` (`cmd/agent`)

Node-side process. Reads `/etc/atellar/agent.json` only — no env vars, no HTTP to control plane.

## Connection

| Field | Use |
|-------|-----|
| `control_plane_address` | Host/IP |
| `http_port` | Not used by agent (atelctl / join register) |
| `grpc_port` | Agent dials `address:grpc_port` |

```go
cfg.ResolveGrpcAddr()  // → "cp-host:9090"
```

## Config example (after join)

```json
{
  "control_plane_address": "cp-host",
  "http_port": 8080,
  "grpc_port": 9090,
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

All three control plane fields are **required** in config.

## Runtime

1. Load config
2. gRPC `Connect` to `control_plane_address:grpc_port`
3. Heartbeat + `reconcile.trigger` handling
4. Overlay reconcile + containerd (when enabled)

## Node setup (atelctl)

- `atelctl agent install` — dirs + systemd
- `atelctl agent join` — writes config (address + ports + credentials)
- `install --auto-join` — install then join in one chain
