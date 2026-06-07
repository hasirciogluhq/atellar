# CLI Usage

Binary: `atellar` (`cmd/cli`)

Handles operator tasks: node registration, agent installation, and API key renewal. **Join is not done by the agent** — it lives in the CLI.

## Commands

### `join`

Registers the machine with the control plane and writes agent config.

```bash
atellar join \
  --token <JOIN_TOKEN> \
  --control-plane-url http://localhost:8080 \
  --name node-1 \
  --public-ip 203.0.113.10 \
  --private-ip 10.0.0.5 \
  --containerd-sock /run/containerd/containerd.sock \
  --heartbeat-interval 5s \
  --config /etc/atellar/agent.json
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--token` | yes | — | Join token |
| `--control-plane-url` | no | `http://localhost:8080` | API base URL |
| `--name` | no | — | Node name (unique) |
| `--public-ip` | no | — | Public IP |
| `--private-ip` | no | — | Private IP |
| `--containerd-sock` | no | `/run/containerd/containerd.sock` | containerd socket |
| `--heartbeat-interval` | no | `5s` | Written to agent config |
| `--config` | no | `/etc/atellar/agent.json` | Output config path |

After registration, `overlay_ip` and `overlay_subnet` are also written to config.

### `install`

Installs the agent binary and creates a systemd service.

```bash
sudo atellar install \
  --agent-bin ./atellar-agent \
  --target /usr/local/bin/atellar-agent \
  --config /etc/atellar/agent.json
```

| Flag | Required | Default |
|------|----------|---------|
| `--agent-bin` | yes | — |
| `--target` | no | `/usr/local/bin/atellar-agent` |
| `--config` | no | `/etc/atellar/agent.json` |

### `renew-api-key`

Renews the node API key (reads key from HTTP or config).

```bash
atellar renew-api-key \
  --config /etc/atellar/agent.json \
  --update-config true
```

## Typical flow

```
join → install → (agent runs automatically)
```

The agent renews the API key automatically before expiry; use `renew-api-key` manually if needed.

## Related code

- `internal/cli/join/`
- `internal/cli/install/`
- `internal/cli/renew/`
- Agent: gRPC only (`internal/grpc/agentclient/`)
- CLI/plugins: HTTP (`internal/pkg/controlplane/`)
