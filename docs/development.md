# Development

## Layout

The repository has two top-level code directories: `cmd/` for the binary and
`pkg/` for everything it is built from.

| Path | Contents |
|------|----------|
| `cmd/atellar` | `main.go` of the single binary |
| `cmd/atellar/internal/cli` | cobra command tree |
| `cmd/atellar/internal/server` | API server process: storage, controllers, HTTP and gRPC listeners |
| `cmd/atellar/internal/agent` | Node agent process: informers, heartbeat, hardware, key renewal |
| `cmd/atellar/internal/workload` | Container lifecycle on a node, restart backoff |
| `cmd/atellar/internal/host` | Files and systemd units written by the install commands |
| `pkg/api`, `pkg/api/watch` | `api.Interface`, error reasons, watch streams |
| `pkg/apis/core/v1` | API types and request validation |
| `pkg/apiserver` | `api.Interface` implementation: authorization, registration, scheduling calls, IPAM calls, events |
| `pkg/apiserver/apiservertest` | Test fixture and the per-method authorization table |
| `pkg/auth` | Token authentication (`authn`), authorization (`authz`), token generation and hashing |
| `pkg/transport` | HTTP and gRPC servers and clients; TCP stub |
| `pkg/client`, `pkg/client/informer` | Client construction from config; list and watch cache |
| `pkg/controller/nodelifecycle` | Marks silent nodes `not_ready` |
| `pkg/scheduler` | Node selection |
| `pkg/events` | In-process publish and subscribe used by watches |
| `pkg/network/ipam` | Node subnet and address pool allocation |
| `pkg/network/overlay` | VXLAN overlay reconciler and its iproute2 dataplane |
| `pkg/network/netns` | Container network namespace setup |
| `pkg/runtime/containerd` | containerd client |
| `pkg/storage` | Storage interface, memory and PostgreSQL implementations, conformance suite |
| `pkg/config` | Server environment, agent file, CLI contexts |

Dependencies point down this table. `pkg/` never imports `cmd/`.

## Tests

```bash
go test -race ./...
```

PostgreSQL tests run when `ATELLAR_TEST_DATABASE_URL` points at an empty
database the tests may truncate; they are skipped otherwise:

```bash
ATELLAR_TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/atellar_test?sslmode=disable' \
  go test -race ./...
```

What the suite covers:

- **Authorization matrix.** `apiservertest.Cases()` lists every API method
  with the expected result for five callers (anonymous, admin, the owning
  node, another node, a bad token). `pkg/apiserver` runs it against the
  implementation; `pkg/transport` runs it over HTTP and gRPC with TLS, so every
  method is checked for every caller on every transport.
- **TLS and CA pinning**: `pkg/pki` covers CA reuse, server certificate
  renewal, pin checks (including a server that shows the real CA but signs
  with another key); the server, CLI and join tests only talk TLS and check
  that wrong pins and foreign CAs are refused. `pkg/pki/pkitest` creates
  throwaway PKI for tests.
- **Error reasons** survive both transports unchanged.
- **Watches**: snapshot, bookmark, live changes and key rotation over both
  transports.
- **Storage conformance**: `pkg/storage/storagetest` runs the same checks
  against the memory store and PostgreSQL.
- **Overlay**: a fake dataplane checks the desired entries for peer sets,
  the write order, the diff (no writes in steady state) and parsing of
  `ip -j` and `bridge -j` output.
- **Workload manager**: a fake runtime and network against a real API server
  over HTTP, covering start, failure, backoff, exit and deletion paths.
- **CLI**: the client commands over HTTP and gRPC against an in-process
  server, and the install commands against a temporary root with a recorded
  `systemctl`.

Code that needs root, containerd or kernel devices (`pkg/runtime/containerd`,
`pkg/network/netns`, the iproute2 dataplane) is tested only through its pure
helpers and fakes.

Benchmarks for the transports:

```bash
go test -run '^$' -bench . ./pkg/transport/
```

## Database changes

Migrations live in `pkg/storage/postgres/migrations` and are applied in order
at server start. Queries live in `pkg/storage/postgres/queries`. After changing
either, regenerate the query code:

```bash
sqlc generate
```

Add the new behavior to `pkg/storage/storagetest` so both stores are held to
it.

## Release

Pushing a `v*` tag runs `.github/workflows/release.yml`, which calls
`scripts/release/package.sh`. It builds `atellar` for linux/amd64 and
linux/arm64 with the version stamped in, packs the migrations and attaches
`install.sh` and `uninstall.sh` to the GitHub release.
