# Getting Started

## Requirements

- Go 1.25+
- PostgreSQL
- (On nodes) containerd — for agent container workloads
- Node `private_ip` values that can reach each other for multi-node experiments (same underlay, VPC, or manually managed WireGuard)

## 1. Install binaries

```bash
# latest release
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash
```

Installs:

- `/usr/local/bin/atellar-api`
- `/usr/local/bin/ateladm`
- `/usr/local/bin/atelctl`
- `/usr/local/bin/atelagent`
- `/usr/share/atellar/migrations`

## 2. Start control plane

```bash
sudo ateladm server install \
  --database-url "postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable" \
  --migrations-path /usr/share/atellar/migrations \
  --port 8080 \
  --grpc-port 9090
```

- HTTP: `http://localhost:8080`
- gRPC: `localhost:9090`
- systemd unit: `atellar-api.service`

## 3. Configure atelctl context

This creates `~/.atellar/config` as JSON.

```bash
atelctl config set-cluster local \
  --control-plane-address 127.0.0.1 \
  --http-port 8080 \
  --grpc-port 9090
atelctl config set-context local --cluster local
atelctl config use-context local
```

## 4. Create join token

```bash
curl -X POST http://localhost:8080/api/v1/nodes/join-tokens \
  -H "Content-Type: application/json" \
  -d '{"single_use": true}'
```

Save the `token` value from the response. It is shown only once.

## 5. Join node

`ateladm node install` creates dirs + systemd unit. With `--auto-join` it registers the node and writes `/etc/atellar/agent.json`.

Run this on the node machine:

```bash
sudo ateladm node install --auto-join \
  --join-token <PLAIN_TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

If no atelctl context exists on the node, pass endpoint flags:

```bash
sudo ateladm node install --auto-join \
  --join-token <PLAIN_TOKEN> \
  --control-plane-address <cp-host> \
  --http-port 8080 \
  --grpc-port 9090 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

`private-ip` is recorded on the node and is the intended data-plane address for multi-node routing. Use the LAN/VPC IP, or your WireGuard IP if you manage WireGuard manually.

## 6. Verify

```bash
# List nodes
atelctl cluster nodes list
# or: curl http://localhost:8080/api/v1/nodes

# Agent logs
journalctl -u atellar-agent.service -f
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

Target node receives `workload.dispatch`; peer nodes receive `container.scheduled` for overlay route reconciliation.

Verify on the control plane:

```bash
atelctl cluster containers list
```

`STATUS` should be `running` with an `OVERLAY_IP`. Same-node container networking is the reliable implemented path today. Cross-node container traffic is still experimental because Atellar does not create a tunnel and does not yet program `private_ip` as the route next hop. If the container is `failed`, check `ERROR` and run diagnostics on the scheduled node — see [networking.md](networking.md).

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
