# Configuration

## API server

`atellar server run` reads its configuration from the environment.
`atellar server install` writes these variables to `/etc/atellar/api.env`
(mode 0600), which the systemd unit loads.

| Variable | Default | Meaning |
|----------|---------|---------|
| `DATABASE_URL` | required | PostgreSQL connection string |
| `MIGRATIONS_PATH` | `/usr/share/atellar/migrations` | SQL migrations, applied at start |
| `BIND_ADDRESS` | empty (all interfaces) | Listen address |
| `PORT` | `8080` | HTTP port |
| `GRPC_PORT` | `9090` | gRPC port; must differ from `PORT` |
| `CLUSTER_OVERLAY_CIDR` | `10.0.0.0/8` | IPv4 range split into node subnets |
| `NODE_SUBNET_PREFIX_LEN` | `24` | Node subnet size, between the cluster prefix and 30 |
| `ADMIN_TOKEN` | required | Admin token |
| `NODE_STALE_AFTER` | `30s` | Heartbeat age after which a node becomes `not_ready` |
| `PKI_DIR` | `/etc/atellar/pki` | Cluster CA and server certificate; missing files are created at start |
| `TLS_SANS` | empty | Comma separated extra DNS names or IPs for the server certificate |

Invalid values stop the server at start with one message per problem.

### PKI

`PKI_DIR` holds four files:

| File | Mode | |
|------|------|---|
| `ca.crt` | 0644 | Cluster CA, valid 10 years. Clients and agents trust only this |
| `ca.key` | 0600 | CA key. Back it up: losing it means re-trusting every node and client |
| `server.crt` | 0644 | Server certificate for `atellar-apiserver` and `TLS_SANS`, valid one year |
| `server.key` | 0600 | Server key |

The server reissues `server.crt` from the CA 30 days before it expires, or at
start when `TLS_SANS` changed, without a restart and without touching the CA.
The CA hash, `sha256:` and the hex SHA-256 of the CA public key, is logged at
start and printed by `atellar server install`.

## Node agent

`/etc/atellar/agent.json`, mode 0600. `atellar node join` writes it; the agent
rewrites it when it renews its API key.

```json
{
  "control_plane": {"address": "10.0.0.10", "http_port": 8080, "grpc_port": 9090},
  "transport": "grpc",
  "ca_cert": "-----BEGIN CERTIFICATE-----\n...",
  "node_id": "node_...",
  "node_name": "node-1",
  "node_api_key": "...",
  "api_key_expires_at": "2027-01-09T12:00:00Z",
  "containerd_sock": "/run/containerd/containerd.sock",
  "heartbeat_interval": "5s",
  "log_dir": "/var/log/atellar",
  "network": {}
}
```

| Field | Default | Meaning |
|-------|---------|---------|
| `control_plane` | required | API server address and ports |
| `transport` | `grpc` | `grpc` or `http` |
| `ca_cert` | required | Cluster CA (PEM), written by `node join` or `node trust-ca` |
| `node_id`, `node_api_key`, `api_key_expires_at` | required | Written by `node join` |
| `containerd_sock` | `/run/containerd/containerd.sock` | containerd socket |
| `heartbeat_interval` | `5s` | Heartbeat period; keep it well below `NODE_STALE_AFTER` |
| `log_dir` | `/var/log/atellar` | Container stdout and stderr |
| `network.underlay_ip` | node private IP | Source address for VXLAN traffic |
| `network.bridge_name` | `atellar0` | Container bridge |
| `network.vxlan_name` | `atellar-vx` | VXLAN device |
| `network.vni` | `4242` | VXLAN network identifier; the same on every node |
| `network.port` | `4789` | VXLAN UDP port; the same on every node |
| `network.mtu` | `1450` | MTU of the bridge, VXLAN device and container veths |
| `network.reconcile_interval` | `30s` | Period of the overlay drift repair |

## CLI

`~/.atellar/config`, mode 0600, managed with `atellar config`.

```json
{
  "current_context": "prod",
  "clusters": {
    "prod": {"address": "10.0.0.10", "http_port": 8080, "grpc_port": 9090, "transport": "grpc", "token": "...", "ca_cert": "-----BEGIN CERTIFICATE-----\n..."}
  },
  "contexts": {
    "prod": {"cluster": "prod"}
  }
}
```

`--config` reads another file and `--context` selects another context for one
call.
