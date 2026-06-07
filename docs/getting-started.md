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

Each GitHub release ships `install.sh` plus binaries (`atellar-api`, `atellar-agent`, `atelctl`) and DB migrations. The installer only copies files — it does not start services.

```bash
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash
# prompts: Kurulacak versiyon (örn. v0.1.0):
```

Or from an extracted tarball:

```bash
tar xzf atellar_0.1.0_linux_amd64.tar.gz
cd atellar_0.1.0_linux_amd64
sudo ./install.sh --local
```

## 4. Start control plane

```bash
export DATABASE_URL="postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable"
export MIGRATIONS_PATH=/usr/share/atellar/migrations
export PORT=8080
export GRPC_PORT=9090
atellar-api
```

## 5. Join agent

`atelctl agent install` creates dirs + systemd unit. With `--auto-join` it registers the node and writes `/etc/atellar/agent.json`.

```bash
sudo atelctl agent install --auto-join \
  --join-token <PLAIN_TOKEN> \
  --control-plane-address <cp-host> \
  --http-port 8080 \
  --grpc-port 9090 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

## 6. Verify

```bash
# List nodes
atelctl cluster nodes list --control-plane-address localhost --http-port 8080 --grpc-port 9090
# or: curl http://localhost:8080/api/v1/nodes

# Agent logs
journalctl -u atellar-agent -f
```

On successful connection, API logs show `agent connected node_id=...`.

## 7. Create container (optional)

Scheduler picks a node automatically — no `node_id` in request body.

```bash
curl -X POST http://localhost:8080/api/v1/containers \
  -H "Content-Type: application/json" \
  -d '{
    "image": "docker.io/library/nginx:alpine"
  }'
```

Target node receives `workload.dispatch`; peer nodes receive `container.scheduled` for overlay routes.

Delete:

```bash
curl -X DELETE http://localhost:8080/api/v1/containers/<CONTAINER_ID>
```

## Adding a second node

Repeat the join flow. If the first node is connected, it receives `node.added` and reconciles.

## Removing a node

```bash
curl -X POST http://localhost:8080/api/v1/nodes/<NODE_ID>/evict
```

The subnet enters the reclaim pool; peers receive `node.removed`.
