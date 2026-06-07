# Architecture

## Overview

Atellar consists of two layers:

1. **Control Plane** — central API server. Node registration, overlay IP allocation, container state, peer notifications.
2. **Agent Node** — runs on each worker machine. Connects to the control plane via gRPC; handles container lifecycle and overlay network reconciliation.

```
┌─────────────┐     HTTP      ┌──────────────────┐
│  atellar    │──────────────►│   API Server     │
│  CLI        │   register    │  :8080 / :9090   │
└─────────────┘               └────────┬─────────┘
                                       │
┌─────────────┐     gRPC bidi          │ PostgreSQL
│ atellar-    │◄──────────────────────┤
│ agent       │  Connect + heartbeat   │
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
- Communicate with control plane **only via gRPC** (no HTTP)

## Auth model

| Phase | Credential | Usage |
|-------|------------|-------|
| Before registration | Join token (plain, shown once) | `POST /nodes/register?token=` |
| After registration | Node API key (Bearer, 90-day TTL) | gRPC stream, `renew-api-key` |

API keys are stored in the DB as SHA-256 hashes.

## Overlay network

- Cluster CIDR: `10.0.0.0/8` (default)
- Node subnet: `/24` (default)
- First host IP in each subnet is the node overlay IP
- Remaining IPs are assigned to containers via `overlay_ip_pool`
- Evicted node subnets are reclaimed on the next registration

## Infrastructure wiring

`cmd/api/shared/infrasturcture.go` creates on startup:

```
Database → sqlc Queries
  → NodeRepository
  → ContainerRepository
  → NodeAuthenticator
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
