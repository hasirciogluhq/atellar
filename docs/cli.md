# atelctl

```
atelctl
├── agent
│   ├── install           # dirs + systemd (+ optional --auto-join → Join chain)
│   ├── join              # register + write /etc/atellar/agent.json
│   └── renew-key
└── cluster
    ├── nodes list
    └── containers list
```

## Control plane connection

Address and ports are **separate and required** on join / cluster commands:

| Flag | Description |
|------|-------------|
| `--control-plane-address` | Host or IP (no scheme) |
| `--http-port` | HTTP API port (atelctl → register, list) |
| `--grpc-port` | gRPC port (agent stream) |

Join writes all three into `/etc/atellar/agent.json`. Agent dials `address:grpc_port`; atelctl HTTP uses `http://address:http_port`.

## Fixed paths

| What | Path |
|------|------|
| Agent binary | `/usr/local/bin/atellar-agent` |
| Agent config | `/etc/atellar/agent.json` |
| Agent logs | `/var/log/atellar` |

## Required join flags

- `--join-token`
- `--name`
- `--public-ip`
- `--private-ip`
- `--control-plane-address`
- `--http-port`
- `--grpc-port`

Same flags required on **`agent install --auto-join`**.

`cluster` commands require: `--control-plane-address`, `--http-port`, `--grpc-port`.

## Flow

```bash
sudo cp atellar-agent /usr/local/bin/

sudo atelctl agent install \
  --auto-join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5 \
  --control-plane-address cp-host \
  --http-port 8080 \
  --grpc-port 9090

# or separate
sudo atelctl agent install
atelctl agent join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5 \
  --control-plane-address cp-host \
  --http-port 8080 \
  --grpc-port 9090
```

## Cluster

```bash
atelctl cluster nodes list \
  --control-plane-address cp-host \
  --http-port 8080 \
  --grpc-port 9090
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

# remove everything (api, agent, atelctl, config, migrations, workloads)
sudo ./uninstall.sh --yes
```

`uninstall.sh` does **not** delete the node from control plane PostgreSQL — use evict API on the CP.

Maintainers: push a `v*` tag — GitHub Actions builds `atellar_<ver>_linux.tar.gz` (amd64 + arm64 inside) and publishes `install.sh`. Local: `./scripts/release/build-all.sh v0.1.0`.

## Related code

- `scripts/release/install.sh`, `scripts/release/uninstall.sh`, `scripts/release/package.sh`
- `pkg/client/controlplane.go` — `ControlPlane`, `HTTPBaseURL()`, `GRPCAddr()`
- `internal/agent/config/` — persisted agent config
