# Atellar

Atellar runs containers across a group of Linux machines. It schedules
containers onto nodes, gives every container a cluster-routable IPv4 address
over a VXLAN overlay, restarts containers that exit, and exposes one API over
HTTP and gRPC.

It is deliberately small: one binary, one PostgreSQL database, containerd on
each node, no custom resource definitions and no plugin system.

## What it does

- **Scheduling** by CPU, memory and free addresses, spreading containers
  across nodes and keeping one active copy of an image per node.
- **Networking**: each node owns a subnet of the cluster range; agents program
  routes, permanent neighbor entries and VXLAN FDB entries from the node list.
  No per-container state leaves the node it runs on.
- **Lifecycle**: image pull, start, exit handling, restart policies (`no`,
  `on-failure`, `always`) with exponential backoff, and a per-container event
  history.
- **Node health**: heartbeats, automatic `not_ready` marking, eviction with
  key revocation and subnet reuse.
- **API**: list, get and watch for nodes and containers; watches deliver a
  snapshot, a bookmark and then live changes. HTTP (JSON, NDJSON streams) and
  gRPC serve the same methods with the same authorization.
- **Auth**: an admin token for operators and per-node API keys for agents,
  hashed at rest, rotated automatically.
- **TLS everywhere**: the API server only speaks TLS with a certificate from
  the cluster CA; nodes and the CLI pin the CA by hash when they first connect.

## Quick start

```bash
# Every machine
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash

# API server (prints the CA hash and the admin token)
sudo atellar server install --database-url "postgresql://atellar:secret@localhost:5432/atellar?sslmode=disable"

# Operator machine
atellar config set-cluster prod --address 10.0.0.10 --ca-cert-hash <CA_HASH> --token <ADMIN_TOKEN>
atellar config set-context prod --cluster prod
atellar config use-context prod
atellar join-tokens create

# Every node (containerd must be running)
sudo atellar node install
sudo atellar node join --join-token <TOKEN> --ca-cert-hash <CA_HASH> --name node-1 --private-ip 10.0.0.21 --control-plane-address 10.0.0.10

# Run something
atellar containers create --image docker.io/library/nginx:1.27 --cpu 0.5 --memory 256
atellar containers list
```

The full walkthrough is in [docs/getting-started.md](docs/getting-started.md).

## The `atellar` binary

| Command | Purpose |
|---------|---------|
| `atellar server install`, `atellar server run` | Install and run the API server |
| `atellar node install`, `node join`, `node renew-key`, `node trust-ca` | Install and register a node agent |
| `atellar agent run` | The node agent process |
| `atellar config ...` | Clusters and contexts for the CLI |
| `atellar nodes ...` | List, inspect, watch and evict nodes |
| `atellar containers ...` | Create, list, inspect, watch and delete containers |
| `atellar join-tokens ...` | Create and list join tokens |

See [docs/cli.md](docs/cli.md).

## Test results

The numbers below come from one run on the current tree: Go 1.25.0,
linux/amd64, 4 vCPU Intel Xeon @ 2.30GHz, PostgreSQL 16.

```
$ ATELLAR_TEST_DATABASE_URL=postgres://... go test -race -count=1 ./...
ok  	github.com/hasirciogluhq/atellar/cmd/atellar/internal/agent
ok  	github.com/hasirciogluhq/atellar/cmd/atellar/internal/cli
ok  	github.com/hasirciogluhq/atellar/cmd/atellar/internal/host
ok  	github.com/hasirciogluhq/atellar/cmd/atellar/internal/server
ok  	github.com/hasirciogluhq/atellar/cmd/atellar/internal/workload
ok  	github.com/hasirciogluhq/atellar/pkg/api
ok  	github.com/hasirciogluhq/atellar/pkg/api/watch
ok  	github.com/hasirciogluhq/atellar/pkg/apis/core/v1
ok  	github.com/hasirciogluhq/atellar/pkg/apiserver
ok  	github.com/hasirciogluhq/atellar/pkg/auth
ok  	github.com/hasirciogluhq/atellar/pkg/auth/authn
ok  	github.com/hasirciogluhq/atellar/pkg/auth/authz
ok  	github.com/hasirciogluhq/atellar/pkg/auth/token
ok  	github.com/hasirciogluhq/atellar/pkg/client
ok  	github.com/hasirciogluhq/atellar/pkg/client/informer
ok  	github.com/hasirciogluhq/atellar/pkg/config/agentconfig
ok  	github.com/hasirciogluhq/atellar/pkg/config/ctlconfig
ok  	github.com/hasirciogluhq/atellar/pkg/config/serverconfig
ok  	github.com/hasirciogluhq/atellar/pkg/controller/nodelifecycle
ok  	github.com/hasirciogluhq/atellar/pkg/events
ok  	github.com/hasirciogluhq/atellar/pkg/network/ipam
ok  	github.com/hasirciogluhq/atellar/pkg/network/netns
ok  	github.com/hasirciogluhq/atellar/pkg/network/overlay
ok  	github.com/hasirciogluhq/atellar/pkg/node/hardware
ok  	github.com/hasirciogluhq/atellar/pkg/runtime/containerd
ok  	github.com/hasirciogluhq/atellar/pkg/scheduler
ok  	github.com/hasirciogluhq/atellar/pkg/storage/memory
ok  	github.com/hasirciogluhq/atellar/pkg/storage/postgres
ok  	github.com/hasirciogluhq/atellar/pkg/transport
ok  	github.com/hasirciogluhq/atellar/pkg/transport/grpc/wire
ok  	github.com/hasirciogluhq/atellar/pkg/transport/http/wire
```

| Measure | Result |
|---------|--------|
| Tests passed (including table subtests) | 907 |
| Tests failed or skipped | 0 |
| Statement coverage, all packages (`-coverpkg=./...`) | 76.3% |
| Authorization matrix | 21 cases covering all 19 API methods, x 5 callers (anonymous, admin, owning node, other node, bad token), x HTTP and gRPC |

Coverage is lowest where code needs root, containerd or kernel devices
(`pkg/runtime/containerd`, `pkg/network/netns`, the iproute2 dataplane); those
are tested through fakes and their pure helpers. How the suite is organized is
described in [docs/development.md](docs/development.md).

### Transport latency

One authenticated `Nodes.Get` call over loopback, including authentication,
authorization and an in-memory store read
(`go test -run '^$' -bench . -benchtime 3s ./pkg/transport/`):

| Benchmark | Time per call | Allocations |
|-----------|---------------|-------------|
| HTTP, one goroutine | 86.6 us | 116 |
| gRPC, one goroutine | 140.0 us | 222 |
| HTTP, 4 goroutines, shared client | 20.2 us | 115 |
| gRPC, 4 goroutines, shared client | 43.5 us | 211 |

These measure the transport and API layers only, not PostgreSQL.

## Documentation

- [Architecture](docs/architecture.md)
- [Getting started](docs/getting-started.md)
- [CLI reference](docs/cli.md)
- [API](docs/api.md)
- [Node agent](docs/agent.md)
- [Networking](docs/networking.md)
- [Configuration](docs/config.md)
- [Development](docs/development.md)

## Status

Atellar is early software. IPv4 only, nodes must share a directly reachable
underlay, and there is no persistent storage for containers yet.

## License

Free for individuals, non-profits, and organizations with annual gross revenue
below **USD 1,000,000**. Organizations at or above that threshold need a
commercial license for production use. This is a source-available license,
not an OSI-approved open source license. See [LICENSE](LICENSE) for the
binding terms.
