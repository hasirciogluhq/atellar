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

## 3. Install agent (atelctl)

On the worker machine — one shot with auto-join:

```bash
go build -o atellar-agent ./cmd/agent
sudo cp atellar-agent /usr/local/bin/

sudo go run ./cmd/atelctl agent install \
  --auto-join \
  --join-token <PLAIN_TOKEN> \
  --control-plane-address <cp-host> \
  --http-port 8080 \
  --grpc-port 9090 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

`install` creates dirs + systemd unit (binary must already be at `/usr/local/bin/atellar-agent`). With `--auto-join` it chains into `join` and writes `/etc/atellar/agent.json`.

Or separately:

```bash
sudo go run ./cmd/atelctl agent install
atelctl agent join \
  --join-token <PLAIN_TOKEN> \
  --control-plane-address <cp-host> \
  --http-port 8080 \
  --grpc-port 9090 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

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
