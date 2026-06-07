# Atellar

Distributed container orchestration platform. Consists of a **control plane** (API server) and **agent** nodes running on each machine.

## Quick start

```bash
# 1. Control plane (requires PostgreSQL)
export DATABASE_URL="postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable"
go run ./cmd/api

# 2. Create join token
curl -X POST http://localhost:8080/api/v1/nodes/join-tokens \
  -H "Content-Type: application/json" \
  -d '{"single_use": true}'

# 3. Install from GitHub release (control plane + agent + atelctl)
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash -s -- \
  --version v0.1.0 \
  --database-url "$DATABASE_URL" \
  --join-token <PLAIN_TOKEN> --name node-1 \
  --public-ip <PUBLIC_IP> --private-ip <PRIVATE_IP> \
  --control-plane-address <CP_HOST> --http-port 8080 --grpc-port 9090
```

## Components

| Component | Binary | Role |
|-----------|--------|------|
| **API Server** | `cmd/api` | HTTP `:8080` + gRPC `:9090`, node/container management, overlay IPAM |
| **atelctl** | `cmd/atelctl` | `agent install/join`, `cluster nodes/containers list` |
| **Agent** | `cmd/agent` | gRPC-only: config, stream, heartbeat, overlay reconcile |

## Architecture (overview)

```
atelctl ──register──► API Server ◄──gRPC stream── Agent
                      │
                   PostgreSQL
                      │
              peer notify (reconcile.trigger)
                      ▼
              other Agents (route/bridge update)
```

On node register, the cluster automatically assigns an **overlay subnet** (`/24`) and **overlay IP**. When containers or nodes change, events are pushed to connected peer nodes.

## Documentation

For detailed usage, config, and architecture:

- [Overview & architecture](docs/architecture.md)
- [Getting started](docs/getting-started.md)
- [atelctl](docs/cli.md)
- [Agent](docs/agent.md)
- [API server](docs/api-server.md)
- [Config & environment](docs/config.md)
- [Peer events (node + container)](docs/peer-events.md)
- [Code layout & dependencies](docs/code-layout.md)

Source: [github.com/hasirciogluhq/atellar](https://github.com/hasirciogluhq/atellar)
