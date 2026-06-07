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

# 3. Join node to cluster
sudo go run ./cmd/cli join --token <PLAIN_TOKEN> --name node-1

# 4. Install and start agent
sudo go run ./cmd/cli install --agent-bin ./atellar-agent
```

## Components

| Component | Binary | Role |
|-----------|--------|------|
| **API Server** | `cmd/api` | HTTP `:8080` + gRPC `:9090`, node/container management, overlay IPAM |
| **CLI** | `cmd/cli` | `join`, `install`, `renew-api-key` |
| **Agent** | `cmd/agent` | Reads config, gRPC stream, heartbeat, peer reconcile events |

## Architecture (overview)

```
CLI ──register──► API Server ◄──gRPC stream── Agent
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
- [CLI usage](docs/cli.md)
- [Agent](docs/agent.md)
- [API server](docs/api-server.md)
- [Config & environment](docs/config.md)
- [Peer events (node + container)](docs/peer-events.md)

Source: [github.com/hasirciogluhq/atellar](https://github.com/hasirciogluhq/atellar)
