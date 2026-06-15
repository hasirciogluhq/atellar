# API Server

Binary: `cmd/api`

Provides the control plane over HTTP (`:8080`) and gRPC (`:9090`).

## Startup

### systemd (production)

```bash
# after release install.sh
sudo ateladm server install \
  --database-url "postgresql://postgres:secret@localhost:5432/atellar_cp?sslmode=disable" \
  --migrations-path /usr/share/atellar/migrations \
  --port 8080 --grpc-port 9090
```

Writes `/etc/atellar/api.env` and enables `atellar-api.service`. Config reference: `cmd/api/.env.example`.

### Docker

```bash
docker compose up --build
```

Or standalone:

```bash
docker build -f cmd/api/Dockerfile -t atellar-api .
docker run --env-file cmd/api/.env.example -p 8080:8080 -p 9090:9090 atellar-api
```

### Local dev

```bash
export DATABASE_URL="postgresql://..."
go run ./cmd/api
```

`main.go` flow:

1. Load config (`envconfig`)
2. Connect PostgreSQL and run migrations
3. `LoadInfrastructure()` — repos, auth, registry, overlay provisioner
4. Start gRPC in a goroutine
5. Gin HTTP router with `/api` prefix

## HTTP routes

All endpoints are under `/api/v1`.

### Nodes `/api/v1/nodes`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/join-tokens` | — | Create join token |
| GET | `/join-tokens` | — | List tokens |
| POST | `/register?token=` | — | Node registration + overlay + API key |
| POST | `/me/api-key/renew` | Bearer | Renew API key |
| POST | `/:nodeId/heartbeat` | — | Heartbeat (HTTP; agent uses gRPC) |
| GET | `` | — | List nodes |
| GET | `/:nodeId` | — | Node details |
| POST | `/:nodeId/evict` | — | Evict node + peer notify |
| PATCH | `/:nodeId/overlay` | — | Update overlay IP/subnet + `node.updated` |

### Containers `/api/v1/containers`

| Method | Path | Description |
|--------|------|-------------|
| POST | `` | Create container → `container.scheduled` |
| GET | `` | List (`?node_id=`) |
| GET | `/:containerId` | Details |
| PATCH | `/:containerId/status` | Update status → peer event |
| PATCH | `/:containerId/runtime` | Update runtime/overlay → peer event |
| GET/POST | `/:containerId/events` | Audit events |

### Overlay IPs `/api/v1/overlay-ips`

| Method | Path | Description |
|--------|------|-------------|
| POST | `` | Add pool entry |
| GET | `` | List (`?node_id=`, `?free=true`) |
| POST | `/:ip/allocate` | Assign to container |
| POST | `/:ip/release` | Release |

## gRPC

Service: `AgentService`

| RPC | Type | Description |
|-----|------|-------------|
| `Connect` | bidi stream | Agent connection, heartbeat, peer push |
| `RenewNodeAPIKey` | unary | API key renewal |
| `GetClusterNetworkState` | unary | Cluster nodes/containers for overlay reconcile |
| `GetNodeWorkloads` | unary | Workloads assigned to authenticated node |
| `ReportContainerRuntime` | unary | Container runtime/status report from node |
| `AllocateContainerOverlayIP` | unary | Allocate overlay IP from authenticated node pool |
| `ReportNodeHardware` | unary | Node hardware report |

On connect, the agent is registered in `AgentRegistry`; on disconnect, unregistered.

## Node register flow

```
Validate token → CreateNode (pending)
  → MarkJoinTokenUsed
  → OverlayProvisioner (reclaim or new /24)
  → IssueNodeAPIKey
  → NotifyNodeAdded (to peers)
```

## Overlay update

```bash
curl -X PATCH http://localhost:8080/api/v1/nodes/<ID>/overlay \
  -H "Content-Type: application/json" \
  -d '{"overlay_ip":"10.0.1.1","overlay_subnet":"10.0.1.0/24"}'
```

If overlay changes, peers receive `node.updated` (with `previous_overlay_*` fields).

## Infrastructure

`internal/controlplane/bootstrap/infrasturcture.go`:

| Component | Role |
|-----------|------|
| `NodeRepository` | Node CRUD, overlay, tokens |
| `ContainerRepository` | Containers + overlay pool |
| `NodeAuth` | Bearer API key validation |
| `Authz` | Scope-based authorization decisions |
| `AgentRegistry` | Connected agent streams |
| `NodePeerNotifier` | Node peer events |
| `ContainerPeerNotifier` | Container peer events |
| `OverlayProvisioner` | IPAM + pool seed |

## Auth note

Most HTTP endpoints are currently unauthenticated (MVP). Production should add admin/service account auth. gRPC and `me/api-key/renew` are protected by node API key and scope-based authz.

## Related code

- `internal/controlplane/transport/http/routes/` — HTTP handlers
- `internal/grpc/server/` — gRPC server
- `internal/grpc/agentregistry/` — peer registry + notify
- `internal/modules/nodes/` — node use cases
- `internal/modules/containers/` — container use cases
