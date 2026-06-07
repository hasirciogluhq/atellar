# atelctl

Binary: `atelctl` (`cmd/atelctl`)

Operator tool for **cluster** (control plane) and **agent** (node) operations.

```
atelctl
├── agent                 # local node agent
│   ├── init              # register node + write config
│   ├── install           # systemd install
│   └── renew-key         # renew node API key
└── cluster               # control plane
    ├── nodes list
    └── containers list
```

## Agent commands

### `agent init`

Registers the machine with the control plane and writes `/etc/atellar/agent.json`.

```bash
atelctl agent init \
  --token <JOIN_TOKEN> \
  --control-plane-url http://localhost:8080 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5 \
  --containerd-sock /run/containerd/containerd.sock \
  --heartbeat-interval 5s \
  --config /etc/atellar/agent.json
```

| Flag | Required | Default |
|------|----------|---------|
| `--token` | yes | — |
| `--control-plane-url` | no | `http://localhost:8080` |
| `--name` | no | — |
| `--public-ip` | no | — |
| `--private-ip` | no | — |
| `--containerd-sock` | no | `/run/containerd/containerd.sock` |
| `--heartbeat-interval` | no | `5s` |
| `--config` | no | `/etc/atellar/agent.json` |

### `agent install`

Installs the agent binary and creates a systemd unit.

```bash
sudo atelctl agent install \
  --agent-bin ./atellar-agent \
  --target /usr/local/bin/atellar-agent \
  --config /etc/atellar/agent.json
```

### `agent renew-key`

Renews the node API key (reads from config by default).

```bash
atelctl agent renew-key --config /etc/atellar/agent.json
```

## Cluster commands

All cluster commands accept `--control-plane-url` (default `http://localhost:8080`).

### `cluster nodes list`

```bash
atelctl cluster nodes list
```

### `cluster containers list`

```bash
atelctl cluster containers list
atelctl cluster containers list --node-id <NODE_ID>
```

## Typical flow

```
atelctl agent init → atelctl agent install → agent runs via systemd
```

The agent renews its API key automatically; use `agent renew-key` manually if needed.

## Related code

- `cmd/atelctl/` — cobra commands
- `internal/atelctl/agent/` — init, install, renew-key
- `internal/atelctl/cluster/` — nodes/containers list
- `internal/client/controlplane/` — HTTP client to control plane
