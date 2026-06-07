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

## 3. Initialize node agent (atelctl)

On the worker machine:

```bash
sudo go run ./cmd/atelctl agent init \
  --token <PLAIN_TOKEN> \
  --control-plane-url http://<cp-host>:8080 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

This command:
1. Calls `POST /api/v1/nodes/register`
2. Assigns overlay subnet/IP
3. Issues a node API key
4. Writes `/etc/atellar/agent.json`

## 4. Install agent

```bash
go build -o atellar-agent ./cmd/agent

sudo go run ./cmd/atelctl agent install \
  --agent-bin ./atellar-agent
```

Creates a systemd unit, enables and restarts the service.

## 5. Verify

```bash
# List nodes
atelctl cluster nodes list
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
