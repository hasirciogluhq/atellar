# Architecture

## Overview

Atellar consists of two layers:

1. **Control Plane** — central API server. Node registration, overlay IP allocation, container state, peer notifications.
2. **Agent Node** — runs on each worker machine. Connects to the control plane via gRPC; handles container lifecycle and overlay network reconciliation.

```
┌─────────────┐     HTTP      ┌──────────────────┐
│  atelctl    │──────────────►│   API Server     │
│ cluster ops │    query      │  :8080 / :9090   │
└─────────────┘               └────────┬─────────┘
┌─────────────┐     HTTP               │
│  ateladm    │──────────────► register│
│ node join   │                        │
└─────────────┘                        │
                                       │
┌─────────────┐     gRPC bidi          │ PostgreSQL
│ atelagent   │◄──────────────────────┤
│             │  Connect + heartbeat   │
└─────────────┘                        │
       ▲                               │
       │ reconcile.trigger             │
       └───────────────────────────────┘
              (push to peer nodes)
```

## Control plane responsibilities

- Join token issuance and node registration
- Per-node overlay subnet/IP allocation (IPAM)
- Reclaiming evicted node subnets
- Container CRUD and lifecycle state
- Async peer event push to connected agents (`reconcile.trigger`)

## Agent responsibilities

- Read `/etc/atellar/agent.json`
- gRPC `Connect` stream (heartbeat, RPC receive)
- Automatic API key renewal (7 days before expiry)
- Apply peer events and reconcile overlay bridge/routes (Linux: `ip` commands)
- Runtime communication with control plane uses gRPC. Registration is performed before daemon start by `ateladm node join` over HTTP.

## Auth model

| Phase | Credential | Usage |
|-------|------------|-------|
| Before registration | Join token (plain, shown once) | `POST /nodes/register?token=` |
| After registration | Node API key (Bearer, 90-day TTL) | gRPC stream, API key renewal |

API keys are stored in the DB as SHA-256 hashes.

## Overlay network

- Cluster CIDR: `10.0.0.0/8` (default)
- Node subnet: `/24` (default)
- First host IP in each subnet is the node overlay IP
- Remaining IPs are assigned to containers via `overlay_ip_pool`
- Evicted node subnets are reclaimed on the next registration

Per-node bridge + per-container veth/netns: [networking.md](networking.md).

Atellar does not create WireGuard or VXLAN. Current code reconciles local bridge state and peer route events, but cross-node traffic is still experimental because the Linux route manager does not yet use peer `private_ip` as the next hop.

## Infrastructure wiring

`internal/controlplane/bootstrap/infrasturcture.go` creates on startup:

```
Database → sqlc Queries
  → NodeRepository
  → ContainerRepository
  → NodeAuthenticator
  → Authorizer
  → AgentRegistry (connected gRPC streams)
  → PeerNotifier (node + container events)
  → OverlayProvisioner (IPAM + pool seed)
```

The gRPC server receives registry and auth via separate `Deps` (avoids import cycles).

## Module structure

Each domain module (nodes, containers) has these layers:

- `domain/` — entities, rules
- `application/usecases/` — workflows
- `application/services/` — cross-cutting services (overlay provisioner)
- `infrastructure/repositories/` — sqlc/PostgreSQL
- `ports/` — interfaces

Full layout and dependency rules: [code-layout.md](code-layout.md)
