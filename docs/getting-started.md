# Getting Started

## Requirements

- Go 1.25+
- PostgreSQL
- (On nodes) containerd — for agent container workloads

## 1. Start control plane

```bash
export DATABASE_URL="postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable"
go run ./cmd/api
```

- HTTP: `http://localhost:8080`
- gRPC: `localhost:9090`

Migrations run automatically.

## 2. Create join token

```bash
curl -X POST http://localhost:8080/api/v1/nodes/join-tokens \
  -H "Content-Type: application/json" \
  -d '{"single_use": true}'
```

Save the `token` value from the response — it is shown only once.

## 3. Install from release

Each GitHub release ships `install.sh` plus binaries (`atellar-api`, `atellar-agent`, `atelctl`) and DB migrations. No build from source on the target machine.

### Control plane + agent on one host

```bash
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/download/v0.1.0/install.sh | sudo bash -s -- \
  --version v0.1.0 \
  --database-url 'postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable' \
  --join-token <PLAIN_TOKEN> \
  --control-plane-address localhost \
  --http-port 8080 \
  --grpc-port 9090 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

### Agent-only node (control plane already running elsewhere)

```bash
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/download/v0.1.0/install.sh | sudo bash -s -- \
  --version v0.1.0 \
  --join-token <PLAIN_TOKEN> \
  --control-plane-address <cp-host> \
  --http-port 8080 \
  --grpc-port 9090 \
  --name node-2 \
  --public-ip 203.0.113.11 \
  --private-ip 10.0.0.6
```

Or extract the release tarball and run locally:

```bash
tar xzf atellar_0.1.0_linux_amd64.tar.gz
cd atellar_0.1.0_linux_amd64
sudo ./install.sh --local --join-token <PLAIN_TOKEN> ...
```

### atelctl only (binaries already installed)

`atelctl agent install` creates dirs + systemd unit (binary must already be at `/usr/local/bin/atellar-agent`). With `--auto-join` it chains into `join` and writes `/etc/atellar/agent.json`.

## 5. Verify

```bash
# List nodes
atelctl cluster nodes list --control-plane-address localhost --http-port 8080 --grpc-port 9090
# or: curl http://localhost:8080/api/v1/nodes

# Agent logs
journalctl -u atellar-agent -f
```

On successful connection, API logs show `agent connected node_id=...`.

## 6. Create container (optional)

```bash
curl -X POST http://localhost:8080/api/v1/containers \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "<NODE_ID>",
    "image": "docker.io/library/nginx:alpine"
  }'
```

Peer nodes receive a `container.scheduled` event.

## Adding a second node

Repeat the join flow. If the first node is connected, it receives `node.added` and reconciles.

## Removing a node

```bash
curl -X POST http://localhost:8080/api/v1/nodes/<NODE_ID>/evict
```

The subnet enters the reclaim pool; peers receive `node.removed`.
