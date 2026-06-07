# Config & Environment

## API server

`internal/config/api.config.go` — loaded via `envconfig`.

| Env | Default | Description |
|-----|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `GRPC_PORT` | `9090` | gRPC port |
| `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `MIGRATIONS_PATH` | `./internal/db/migrations` | Migration directory |
| `CLUSTER_OVERLAY_CIDR` | `10.0.0.0/8` | Cluster overlay network |
| `NODE_SUBNET_PREFIX_LEN` | `24` | Per-node subnet prefix |

Example `.env`:

```bash
DATABASE_URL=postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable
CLUSTER_OVERLAY_CIDR=10.0.0.0/8
NODE_SUBNET_PREFIX_LEN=24
```

`main.go` sets a local default if `DATABASE_URL` is empty (dev convenience).

## Agent

No environment variables. Single source: `/etc/atellar/agent.json`

Details: [agent.md](agent.md)

## Post-join config example

```json
{
  "control_plane_address": "10.0.0.1",
  "http_port": 8080,
  "grpc_port": 9090,
  "node_id": "a1b2c3d4-...",
  "node_name": "worker-1",
  "overlay_ip": "10.0.0.1",
  "overlay_subnet": "10.0.0.0/24",
  "node_api_key": "abc123...",
  "api_key_expires_at": "2026-09-05T10:00:00Z",
  "containerd_sock": "/run/containerd/containerd.sock",
  "heartbeat_interval": "5s",
  "bridge_name": "atellar0",
  "reconcile_interval": "30s"
}
```

## Token TTL

| Credential | TTL | Renew threshold |
|------------|-----|-----------------|
| Node API key | 90 days | 7 days before expiry (agent auto) |
| Join token | Set at creation | — |

## Proto generate

```bash
./scripts/generate-proto.sh
```

## sqlc generate

```bash
sqlc generate
```
