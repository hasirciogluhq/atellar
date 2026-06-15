# Atellar

Distributed container orchestration platform. Consists of a **control plane** (API server) and **agent** nodes running on each machine.

## Quick start

```bash
# 1. Install release binaries and migrations
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash

# 2. Install and start the control plane (requires PostgreSQL)
sudo ateladm server install \
  --database-url "postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable" \
  --migrations-path /usr/share/atellar/migrations \
  --port 8080 \
  --grpc-port 9090

# 3. Save a local client context
atelctl config set-cluster local --control-plane-address 127.0.0.1 --http-port 8080 --grpc-port 9090
atelctl config set-context local --cluster local
atelctl config use-context local

# 4. Create join token
curl -X POST http://localhost:8080/api/v1/nodes/join-tokens \
  -H "Content-Type: application/json" \
  -d '{"single_use": true}'

# 5. Join a node from that node machine
sudo ateladm node install --auto-join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip <PUBLIC_IP> \
  --private-ip <PRIVATE_OR_WG_IP>

# 6. Inspect from your client machine
atelctl cluster nodes list
```

## Components

| Component | Binary | Role |
|-----------|--------|------|
| **API Server** | `cmd/api` | HTTP `:8080` + gRPC `:9090`, node/container management, overlay IPAM |
| **atelctl** | `cmd/atelctl` | Client config, contexts, cluster inspection |
| **ateladm** | `cmd/ateladm` | Server install and node join/install operations |
| **Agent** | `cmd/atelagent` | gRPC-only: config, stream, heartbeat, overlay reconcile |

## Architecture (overview)

```
atelctl ──query──────► API Server ◄──gRPC stream── atelagent
ateladm ──manage─────►     ▲
                      │
                   PostgreSQL
                      │
              peer notify (reconcile.trigger)
                      ▼
              other Agents (route/bridge update)
```

On node register, the cluster automatically assigns an **overlay subnet** (`/24`) and **overlay IP**. When containers or nodes change, events are pushed to connected peer nodes.

Current networking creates the per-node bridge, container netns/veth pairs, overlay IPAM, and peer route reconciliation triggers. Atellar does **not** create VXLAN, WireGuard, or another node-to-node tunnel. Cross-node container traffic should be treated as experimental until route reconciliation uses node `private_ip` as the data-plane next hop. For multi-node tests, keep node `private_ip` values mutually reachable; use same LAN/VPC addresses or WireGuard IPs that you manage outside Atellar.

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
