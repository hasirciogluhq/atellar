# Configuration

## API server

`atellar server run` reads its configuration from the environment.
`atellar server install` writes these variables to `/etc/atellar/api.env`
(mode 0600), which the systemd unit loads.

| Variable | Default | Meaning |
|----------|---------|---------|
| `DATABASE_URL` | required | PostgreSQL connection string |
| `MIGRATIONS_PATH` | `/usr/share/atellar/migrations` | SQL migrations, applied at start |
| `BIND_ADDRESS` | empty (all interfaces) | Listen address |
| `PORT` | `8080` | HTTP port |
| `GRPC_PORT` | `9090` | gRPC port; must differ from `PORT` |
| `CLUSTER_OVERLAY_CIDR` | `10.0.0.0/8` | IPv4 range split into node subnets |
| `NODE_SUBNET_PREFIX_LEN` | `24` | Node subnet size, between the cluster prefix and 30 |
| `ADMIN_TOKEN` | empty | Admin token. Empty runs the cluster open: every unauthenticated caller is admin |
| `NODE_STALE_AFTER` | `30s` | Heartbeat age after which a node becomes `not_ready` |

Invalid values stop the server at start with one message per problem.

## Node agent

`/etc/atellar/agent.json`, mode 0600. `atellar node join` writes it; the agent
rewrites it when it renews its API key.

```json
{
  "control_plane": {"address": "10.0.0.10", "http_port": 8080, "grpc_port": 9090},
  "transport": "grpc",
  "node_id": "node_...",
  "node_name": "node-1",
  "node_api_key": "...",
  "api_key_expires_at": "2027-01-09T12:00:00Z",
  "containerd_sock": "/run/containerd/containerd.sock",
  "heartbeat_interval": "5s",
  "log_dir": "/var/log/atellar",
  "network": {}
}
```

| Field | Default | Meaning |
|-------|---------|---------|
| `control_plane` | required | API server address and ports |
| `transport` | `grpc` | `grpc` or `http` |
| `node_id`, `node_api_key`, `api_key_expires_at` | required | Written by `node join` |
| `containerd_sock` | `/run/containerd/containerd.sock` | containerd socket |
| `heartbeat_interval` | `5s` | Heartbeat period; keep it well below `NODE_STALE_AFTER` |
| `log_dir` | `/var/log/atellar` | Container stdout and stderr |
| `network.underlay_ip` | node private IP | Source address for VXLAN traffic |
| `network.bridge_name` | `atellar0` | Container bridge |
| `network.vxlan_name` | `atellar-vx` | VXLAN device |
| `network.vni` | `4242` | VXLAN network identifier; the same on every node |
| `network.port` | `4789` | VXLAN UDP port; the same on every node |
| `network.mtu` | `1450` | MTU of the bridge, VXLAN device and container veths |
| `network.reconcile_interval` | `30s` | Period of the overlay drift repair |

## CLI

`~/.atellar/config`, mode 0600, managed with `atellar config`.

```json
{
  "current_context": "prod",
  "clusters": {
    "prod": {"address": "10.0.0.10", "http_port": 8080, "grpc_port": 9090, "transport": "grpc", "token": "..."}
  },
  "contexts": {
    "prod": {"cluster": "prod"}
  }
}
```

`--config` reads another file and `--context` selects another context for one
call.
