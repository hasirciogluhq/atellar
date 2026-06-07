# atelctl

```
atelctl
├── agent
│   ├── install           # dirs + systemd (optional --auto-join)
│   ├── join              # register + write /etc/atellar/agent.json
│   └── renew-key
└── cluster
    ├── nodes list
    └── containers list
```

Fixed paths (no flags):
- Agent binary: `/usr/local/bin/atellar-agent`
- Config: `/etc/atellar/agent.json`
- Logs: `/var/log/atellar`

## Flow

```bash
# 1. put binary on the node
sudo cp atellar-agent /usr/local/bin/

# 2. install service (+ optional join)
sudo atelctl agent install \
  --auto-join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5

# or separately:
sudo atelctl agent install
atelctl agent join \
  --join-token <TOKEN> \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5
```

## `agent install`

Creates `/etc/atellar`, `/var/log/atellar`, writes `atellar-agent.service`, enables it.

Requires `atellar-agent` already at `/usr/local/bin/atellar-agent`.

| Flag | Description |
|------|-------------|
| `--auto-join` | join after install |
| `--control-plane-url` | default `http://localhost:8080` |

With `--auto-join`, these are **required**: `--join-token`, `--name`, `--public-ip`, `--private-ip`.

## `agent join`

Writes full config to `/etc/atellar/agent.json` and restarts the agent service.

**Required:** `--join-token`, `--name`, `--public-ip`, `--private-ip`

## Cluster

```bash
atelctl cluster nodes list
atelctl cluster containers list --node-id <ID>
```
