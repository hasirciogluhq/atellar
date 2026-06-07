# Atellar Documentation

## Contents

| Document | Description |
|----------|-------------|
| [architecture.md](architecture.md) | System architecture, components, data flow |
| [getting-started.md](getting-started.md) | First-time setup and end-to-end flow |
| [cli.md](cli.md) | `atelctl` — agent & cluster commands |
| [agent.md](agent.md) | Agent behavior and config |
| [api-server.md](api-server.md) | HTTP/gRPC API, routes, infrastructure |
| [config.md](config.md) | Environment variables and config files |
| [peer-events.md](peer-events.md) | Node and container peer notification events |
| [code-layout.md](code-layout.md) | Folder structure and dependency rules |

## Repository layout (summary)

```
cmd/
  api/          Control plane
  atelctl/      Operator CLI (agent + cluster)
  agent/        Node agent
internal/
  modules/nodes/       Node domain + use cases
  modules/containers/  Container domain + use cases
  grpc/                gRPC server, peer registry
  agent/               config, grpcclient, overlay
  cluster/ipam/        control plane overlay IPAM
  platform/            authn, pgutil, tokenhash
  client/controlplane/ HTTP client for CLI/plugins
api/proto/             Protobuf definitions
```
