# Agent

Binary: `atelagent` (`cmd/atelagent`)

Node-side process. Reads `/etc/atellar/agent.json` only. Runtime communication with the control plane is gRPC; node registration is done separately by `ateladm node join` over HTTP before the daemon starts.

## Connection

| Field | Use |
|-------|-----|
| `control_plane_address` | Host/IP |
| `http_port` | Written for tooling; atelagent runtime does not use it |
| `grpc_port` | Agent dials `address:grpc_port` |

```go
cfg.ResolveGrpcAddr()  // → "cp-host:9090"
```

## gRPC API (agent → control plane)

| RPC | Purpose |
|-----|---------|
| `Connect` | Heartbeat + `reconcile.trigger` stream |
| `GetNodeWorkloads` | Poll workloads for this node (15s) |
| `ReportContainerRuntime` | Report status at every lifecycle step |
| `AllocateContainerOverlayIP` | Request overlay IP after containerd create |
| `ReportNodeHardware` | CPU/RAM/disk/hostname on start + every 10m |
| `RenewNodeAPIKey` | Token renewal |

Push events on stream: `workload.dispatch`, `workload.removed` (trigger immediate reconcile).

## Container runtime pipeline

1. `pulling` — image pull (skip if digest matches)
2. `creating` — prepare containerd container
3. `AllocateContainerOverlayIP` — CP assigns overlay IP from node pool
4. veth + netns setup (`internal/agent/netns`) — uses `overlay_ip` / `overlay_subnet` from agent config as gateway and address prefix
5. `running` — containerd task start + report PID/digest

`runtime.NewManager` receives `cfg.OverlayIP` and `cfg.OverlaySubnet` from `agent.json` and passes them to `netns.Setup` as `NodeOverlayIP` and `NodeOverlaySubnet`. Without a valid node overlay IP on the bridge, default-route setup fails (`Nexthop has invalid gateway`). See [networking.md](networking.md).

On failure: `backoff` with exponential delay (max 5 retries) → `failed`. CLI `containers list` shows the `ERROR` column.

On `DELETE /containers/:id`: CP sets `removed` → agent terminates, cleans netns/containerd, reports `terminated`.

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

## Node setup (ateladm)

- `ateladm node install` — dirs + systemd
- `ateladm node join` — writes config (address + ports + credentials)
- `install --auto-join` — install then join in one chain
