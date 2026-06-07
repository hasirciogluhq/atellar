# Atellar Documentation

## Contents

| Document | Description |
|----------|-------------|
| [architecture.md](architecture.md) | System architecture, components, data flow |
| [getting-started.md](getting-started.md) | First-time setup and end-to-end flow |
| [cli.md](cli.md) | `atellar` CLI commands |
| [agent.md](agent.md) | Agent behavior and config |
| [api-server.md](api-server.md) | HTTP/gRPC API, routes, infrastructure |
| [config.md](config.md) | Environment variables and config files |
| [peer-events.md](peer-events.md) | Node and container peer notification events |

## Repository layout (summary)

```
cmd/
  api/          Control plane
  cli/          Operator CLI
  agent/        Node agent
internal/
  modules/nodes/       Node domain + use cases
  modules/containers/  Container domain + use cases
  grpc/                gRPC server, agent client, peer registry
  pkg/                 agentconfig, authn, overlayipam, overlaynet, ...
api/proto/             Protobuf definitions
```
