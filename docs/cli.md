# CLI

```
atelctl
├── cluster
    ├── nodes list
    └── containers list
├── config
    ├── set-cluster
    ├── set-context
    ├── use-context
    ├── current-context
    └── get-contexts

ateladm
├── server
│   └── install           # /etc/atellar/api.env + atellar-api.service
└── node
    ├── install           # dirs + systemd (+ optional --auto-join)
    ├── join              # register + write /etc/atellar/agent.json
    └── renew-key

atelagent                       # node agent daemon
```

## Control plane connection

Address and ports can be stored in an atelctl context, so day-to-day commands do not need endpoint flags.

| Flag | Description |
|------|-------------|
| `--control-plane-address` | Host or IP (no scheme) |
| `--http-port` | HTTP API port (atelctl → register, list) |
| `--grpc-port` | gRPC port (agent stream) |

Explicit endpoint flags still work and override the current context. `ateladm node join` writes all three resolved values into `/etc/atellar/agent.json`. atelagent dials `address:grpc_port`; atelctl HTTP uses `http://address:http_port`.

## Config

Default path: `~/.atellar/config`. Override with global `--config`.

```bash
atelctl config set-cluster local \
  --control-plane-address 127.0.0.1 \
  --http-port 8080 \
  --grpc-port 9090

atelctl config set-context local --cluster local
atelctl config use-context local

atelctl config current-context
atelctl config get-contexts
```

## Fixed paths

| What | Path |
|------|------|
| API binary | `/usr/local/bin/atellar-api` |
| API env (systemd) | `/etc/atellar/api.env` |
| API migrations | `/usr/share/atellar/migrations` |
| Agent binary | `/usr/local/bin/atelagent` |
| Agent config | `/etc/atellar/agent.json` |
| Agent logs | `/var/log/atellar` |

## server install

Requires root. Writes secrets to `/etc/atellar/api.env` and installs `atellar-api.service`.

```bash
sudo ateladm server install \
  --database-url "postgresql://postgres:secret@localhost:5432/atellar_cp?sslmode=disable" \
  --migrations-path /usr/share/atellar/migrations \
  --port 8080 --grpc-port 9090
```

| Flag | Default | Description |
|------|---------|-------------|
| `--database-url` | *(required)* | PostgreSQL DSN |
| `--migrations-path` | `/usr/share/atellar/migrations` | SQL migrations dir |
| `--port` | `8080` | HTTP port |
| `--grpc-port` | `9090` | gRPC port |
| `--cluster-overlay-cidr` | `10.0.0.0/8` | Overlay CIDR |
| `--node-subnet-prefix-len` | `24` | Per-node subnet prefix |
| `--start` | `true` | `systemctl restart` after install |

Env template: `cmd/api/.env.example`

## Required join flags

- `--join-token`
- `--name`
- `--public-ip`
- `--private-ip`

Same flags are required on **`ateladm node install --auto-join`**.

Endpoint flags are optional when a current context is configured:

- `--control-plane-address`
- `--http-port`
- `--grpc-port`

`cluster` commands use the current context by default.

## Flow

```bash
sudo cp atelagent /usr/local/bin/

sudo ateladm node install \
  --auto-join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5

# or separate
sudo ateladm node install
ateladm node join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

## Cluster

```bash
atelctl cluster nodes list
atelctl cluster containers list
```

## Release install / uninstall

Each GitHub release includes `install.sh` and `uninstall.sh` (also under `scripts/release/` in the repo).

`install.sh` only installs binaries and migrations. Auto-detects `linux/amd64` or `linux/arm64` from the universal tarball. Does not start services.

```bash
# from a release tag (version baked in — no prompt)
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/download/v0.1.0/install.sh | sudo bash

# latest release
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash

# from extracted tarball
sudo ./install.sh --local

# remove everything (api, agent, ateladm, atelctl, config, migrations, workloads)
sudo ./uninstall.sh --yes
```

`uninstall.sh` does **not** delete the node from control plane PostgreSQL — use evict API on the CP.

Maintainers: push a `v*` tag — GitHub Actions builds `atellar_<ver>_linux.tar.gz` (amd64 + arm64 inside) and publishes `install.sh`. Local: `./scripts/release/build-all.sh v0.1.0`.

## Related code

- `scripts/release/install.sh`, `scripts/release/uninstall.sh`, `scripts/release/package.sh`
- `pkg/client/controlplane.go` — `ControlPlane`, `HTTPBaseURL()`, `GRPCAddr()`
- `internal/agent/config/` — persisted agent config
