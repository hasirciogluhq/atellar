# atelctl

Binary: `atelctl` (`cmd/atelctl`)

Operator tool for **cluster** (control plane) and **agent** (node) operations.

```
atelctl
├── agent
│   ├── init              # prepare node locally (dirs, containerd check)
│   ├── join              # register with control plane + write config
│   ├── install           # systemd install
│   └── renew-key
└── cluster
    ├── nodes list
    └── containers list
```

HTTP calls go through `pkg/client.AtellarClient`.

## Agent flow

```
atelctl agent init → atelctl agent join → atelctl agent install
```

### `agent init`

Prepares the node — no control plane call.

```bash
sudo atelctl agent init \
  --containerd-sock /run/containerd/containerd.sock \
  --config-dir /etc/atellar \
  --log-dir /var/log/atellar
```

Creates config/log directories and verifies containerd socket.

### `agent join`

Registers with the control plane and writes `/etc/atellar/agent.json`.

```bash
atelctl agent join \
  --token <JOIN_TOKEN> \
  --control-plane-url http://localhost:8080 \
  --name node-1
```

### `agent install`

```bash
sudo atelctl agent install --agent-bin ./atellar-agent
```

### `agent renew-key`

```bash
atelctl agent renew-key --config /etc/atellar/agent.json
```

## Cluster commands

```bash
atelctl cluster nodes list
atelctl cluster containers list --node-id <NODE_ID>
```

## pkg/client

Global HTTP API client used by atelctl and external tools:

```go
api := client.New(client.Options{BaseURL: "http://localhost:8080"})
api.RegisterNode(ctx, joinToken, client.RegisterNodeRequest{...})
api.ListNodes(ctx)
```

Service account auth: `ATELLAR_SERVICE_ACCOUNT_SECRET` / `ATELLAR_SERVICE_ACCOUNT_TOKEN` or `/var/run/secrets/atellar/service-account/secret`.
