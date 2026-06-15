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

Example env file: `cmd/api/.env.example`

Production (systemd) — written by `ateladm server install` to `/etc/atellar/api.env` (mode `0600`):

```bash
sudo ateladm server install \
  --database-url "postgresql://postgres:secret@localhost:5432/atellar_cp?sslmode=disable" \
  --migrations-path /usr/share/atellar/migrations \
  --port 8080 --grpc-port 9090
```

Docker: `docker compose up --build` or `docker build -f cmd/api/Dockerfile -t atellar-api .` with env from `cmd/api/.env.example`.

`main.go` sets a local default if `DATABASE_URL` is empty (dev convenience only).

## Agent

No environment variables. Single source: `/etc/atellar/agent.json`

Details: [agent.md](agent.md)

**Container networking requires** `overlay_ip` and `overlay_subnet` (set at join/register). They must match the bridge address on `bridge_name` (default `atellar0`). Troubleshooting: [networking.md](networking.md).

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
