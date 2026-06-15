# Code Layout

Dependency direction is from entrypoints and transports into application/domain code. Domain packages must not import transport, CLI, or agent runtime packages.

```
cmd/
  api/              Control plane API server entrypoint
  atelctl/          Client CLI entrypoint
  ateladm/          Admin/operator CLI entrypoint
  atelagent/        Node agent daemon entrypoint

internal/
  controlplane/
    bootstrap/      Database, repositories, authn/authz, registry wiring
    transport/http/ Gin HTTP handlers and middleware

  grpc/
    gen/            Generated AgentService protobuf code
    server/         Control-plane gRPC implementation for atelagent
    agentregistry/ Connected-agent registry and peer push

  modules/
    nodes/          Node domain, use cases, ports, repositories
    containers/     Container domain, use cases, ports, repositories

  agent/            Code that runs inside atelagent
    config/         /etc/atellar/agent.json schema
    grpcclient/     Agent gRPC session and control-plane sync
    runtime/        Containerd lifecycle manager
    netns/          Linux netns/veth setup
    overlay/        atellar0 bridge and route reconcile
    hardware/       Node hardware reporter

  cli/
    atelctl/        Client command application services
    ateladm/        Admin command application services

  cluster/ipam/     Control-plane overlay subnet and IP allocation
  config/           API server env config
  db/               migrations, sqlc queries, generated SQL
  platform/         authn, authz, pgutil, tokenhash, nodetoken

pkg/
  client/           Public HTTP API client with its own DTOs

api/proto/          Source protobuf definitions
scripts/release/   Build/install/uninstall release scripts
```

## Binary Responsibilities

| Binary | Package | Role |
|--------|---------|------|
| `atellar-api` | `cmd/api` | Control plane HTTP and gRPC server |
| `atelctl` | `cmd/atelctl` | User/client CLI, contexts, cluster inspection |
| `ateladm` | `cmd/ateladm` | Server install and node install/join operations |
| `atelagent` | `cmd/atelagent` | Node daemon: gRPC, containerd, netns, overlay |

## Dependency Rules

| Layer | May import | Must not import |
|-------|------------|-----------------|
| `cmd/*` | `internal/*`, `pkg/*` | Business logic directly |
| `controlplane/transport/*` | `controlplane/bootstrap`, `modules/*`, `platform/*` | `agent/*`, `cli/*` |
| `grpc/server` | `modules/*`, `platform/*`, `grpc/agentregistry` | `agent/*`, `cli/*` |
| `modules/*` | `platform/*`, `db/*`, `cluster/*` | `agent/*`, transport packages |
| `agent/*` | `grpc/gen`, `platform/authn`, agent-local packages | `modules/*`, control-plane repositories |
| `cli/*` | `pkg/client`, install/join helpers | `controlplane/transport/*` |
| `pkg/client` | stdlib, jwt | `internal/*` |

## Transport Split

| Component | Transport |
|-----------|-----------|
| `atelctl` / `ateladm` | HTTP via `pkg/client` |
| `atelagent` | gRPC to `AgentService` |
| Container traffic | Linux bridge/netns plus node-to-node routed reachability |

Do not put Linux networking code under `pkg/` or `platform/`. It belongs under `internal/agent/`.
