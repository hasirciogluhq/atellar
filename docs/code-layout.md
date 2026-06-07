# Code Layout

Dependency direction is **top → down**. Lower layers must not import domain modules.

```
cmd/                          # binaries (thin entrypoints)
├── api/                      # control plane HTTP + wiring
├── agent/                    # node agent binary
└── atelctl/                  # operator CLI (agent + cluster)

internal/
├── config/                   # API server env config
├── db/                       # migrations, sqlc queries, generated SQL

├── modules/                  # domain-driven control plane features
│   ├── nodes/                # domain → usecases → ports → infrastructure
│   └── containers/

├── cluster/                  # cluster-wide control plane logic (no agent code)
│   └── ipam/                 # overlay subnet/IP allocation

├── transport/                # (conceptual) — today: internal/grpc/
│   └── grpc/
│       ├── gen/              # protobuf
│       ├── server/           # AgentService (control plane side)
│       └── agentregistry/    # connected agent streams + peer push

├── agent/                    # everything that runs ON a node
│   ├── agent.go              # Run()
│   ├── config/               # agent.json schema
│   ├── grpcclient/           # gRPC session to control plane (agent-only)
│   └── overlay/              # bridge, routes, reconcile (linux + stub)
│       ├── manager.go        # reconcile loop
│       ├── state.go          # desired routes (no domain imports)
│       ├── cluster.go        # ClusterNode/Container DTOs + ClusterSyncer
│       ├── link_linux.go     # ip route/link (linux)
│       └── link_stub.go      # dev/macOS stub

├── client/                   # outbound HTTP clients (CLI, plugins — NOT agent)
│   └── controlplane/

├── platform/                 # shared primitives (authn, pgutil, hashes)
│   ├── authn/
│   ├── pgutil/
│   ├── tokenhash/
│   └── nodetoken/

└── atelctl/                  # atelctl command implementations
    ├── agent/                # init, install, renew-key
    └── cluster/              # nodes/containers list
```

## Dependency rules

| Layer | May import | Must NOT import |
|-------|------------|-----------------|
| `cmd/*` | `internal/*` | — |
| `modules/*` | `platform/*`, `db/*`, `cluster/*` | `agent/*`, `grpc/server` |
| `cluster/ipam` | stdlib only | `modules/*`, `agent/*` |
| `agent/overlay` | stdlib only | `modules/*`, `client/*` |
| `agent/grpcclient` | `agent/config`, `agent/overlay`, `platform/authn`, `grpc/gen` | `modules/*` |
| `grpc/server` | `modules/*`, `platform/*`, `grpc/agentregistry` | `agent/*` |
| `client/controlplane` | `modules/domain` (DTOs for JSON) | `agent/*` |

## What was removed

- `internal/pkg/*` — dumping ground; split by responsibility
- `internal/nodes/` — duplicate of `modules/nodes`
- `internal/network/` — empty stub

## Agent vs HTTP

| Component | Transport |
|-----------|-----------|
| `agent/grpcclient` | gRPC only |
| `agent/overlay` | local `ip` commands (linux) |
| `client/controlplane` | HTTP (atelctl, external plugins) |
| `cmd/api` routes | HTTP REST |

## Future (when adding vmnet / container netns)

Keep under `internal/agent/`:

```
agent/
  overlay/          # node overlay bridge + cluster routes (existing)
  netns/            # per-container veth, netns setup (future)
  vmnet/            # optional VM bridge/tap (future)
```

Do **not** put Linux network code under `pkg/` or `platform/`. Those stay agent-local or `cluster/` for control-plane IPAM only.
