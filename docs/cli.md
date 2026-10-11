# CLI reference

Atellar ships one binary, `atellar`. It installs the API server and node
agent, runs both as systemd services, and is the client for every API call.

```
atellar [--config PATH] [--context NAME] [-o table|json] <command>
```

| Global flag | Default | Meaning |
|-------------|---------|---------|
| `--config` | `~/.atellar/config` | CLI configuration file |
| `--context` | current context | Use another context for this call |
| `-o, --output` | `table` | `table` or `json` |

## Setup commands (run as root)

### `atellar server install`

Creates the cluster CA and server certificate in the PKI directory (kept if
they exist), writes `/etc/atellar/api.env` (mode 0600) and installs, enables
and starts `atellar-server.service`. Prints the CA hash and the admin token.

| Flag | Default | |
|------|---------|---|
| `--database-url` | required | PostgreSQL connection string |
| `--migrations-path` | `/usr/share/atellar/migrations` | SQL migrations |
| `--bind-address` | all interfaces | Listen address |
| `--http-port` | `8080` | HTTP port |
| `--grpc-port` | `9090` | gRPC port |
| `--cluster-cidr` | `10.0.0.0/8` | IPv4 range split into node subnets |
| `--node-subnet-prefix` | `24` | Prefix length of each node subnet |
| `--node-stale-after` | `30s` | Heartbeat age that marks a node `not_ready` |
| `--admin-token` | generated | Admin token |
| `--pki-dir` | `/etc/atellar/pki` | Cluster CA and server certificate |
| `--tls-san` | | Extra DNS name or IP for the server certificate, repeatable |
| `--start` | `true` | Start the service after installing |

### `atellar node install`

Installs and enables `atellar-agent.service`. The service starts once
`node join` has written the agent configuration.

### `atellar node join`

Fetches the cluster CA and checks it against `--ca-cert-hash` (or reads it
from `--ca-cert`), registers the machine with the API server, writes
`/etc/atellar/agent.json` (mode 0600) and restarts the agent if its unit is
installed.

| Flag | Default | |
|------|---------|---|
| `--join-token` | required | Token from `atellar join-tokens create` |
| `--ca-cert-hash` | | Cluster CA pin (`sha256:...`); this or `--ca-cert` is required |
| `--ca-cert` | | Cluster CA certificate file, instead of fetching it |
| `--name` | required | Node name, unique in the cluster |
| `--private-ip` | required | Address other nodes reach this node on; carries VXLAN traffic |
| `--control-plane-address` | required | API server host or IP |
| `--http-port` | `8080` | API server HTTP port |
| `--grpc-port` | `9090` | API server gRPC port |
| `--transport` | `grpc` | Transport the agent uses: `grpc` or `http` |
| `--public-ip` | | Informational |
| `--containerd-sock` | `/run/containerd/containerd.sock` | containerd socket |
| `--heartbeat-interval` | `5s` | Agent heartbeat interval |

### `atellar node renew-key`

Requests a new node API key, writes it to the agent configuration and restarts
the agent. The agent also renews its key on its own seven days before expiry.

### `atellar node trust-ca`

Stores the cluster CA in `/etc/atellar/agent.json` and restarts the agent.
Takes `--ca-cert-hash` or `--ca-cert` like `node join`. Nodes joined before the
API server served TLS run it once.

### `atellar server run`, `atellar agent run`

The processes systemd starts. `server run` reads its configuration from the
environment (see [config.md](config.md)), applies migrations and serves.
`agent run --agent-config PATH` runs the node agent.

## Client configuration

```bash
atellar config set-cluster NAME --address HOST (--ca-cert-hash sha256:... | --ca-cert FILE) [--http-port 8080] [--grpc-port 9090] [--transport grpc|http] [--token TOKEN]
atellar config set-context NAME --cluster CLUSTER
atellar config use-context NAME
atellar config current-context
atellar config get-contexts
```

`set-cluster` stores the cluster CA in the configuration. With
`--ca-cert-hash` it fetches the CA from the API server and refuses to save
the cluster when the hash does not match.

## Operating the cluster

| Command | Does |
|---------|------|
| `atellar nodes list` | List nodes |
| `atellar nodes get NODE` | Show one node (ID or name) |
| `atellar nodes evict NODE` | Evict a node, revoke its key, free its subnet |
| `atellar nodes overlay-ips NODE [--free]` | List a node's overlay addresses |
| `atellar nodes watch` | Stream node changes |
| `atellar join-tokens create [--ttl 24h] [--multi-use]` | Create a join token |
| `atellar join-tokens list` | List join tokens |
| `atellar containers create --image REF [flags]` | Schedule a container |
| `atellar containers list [--node ID] [--active]` | List containers |
| `atellar containers get ID` | Show one container |
| `atellar containers delete ID` | Stop and remove a container |
| `atellar containers events ID` | Lifecycle history, newest first |
| `atellar containers watch [--node ID]` | Stream container changes |
| `atellar version` | Print the version |

`containers create` flags:

| Flag | Default | |
|------|---------|---|
| `--image` | required | Image reference, e.g. `docker.io/library/nginx:1.27` |
| `--cpu` | none | CPU limit in cores |
| `--memory` | none | Memory limit in MiB |
| `--env KEY=VALUE` | | Repeatable |
| `--command` | image CMD | Arguments that replace the image CMD |
| `--restart` | `on-failure` | `no`, `on-failure` or `always` |
