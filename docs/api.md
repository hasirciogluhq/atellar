# API

The API is defined once in `pkg/api` (`api.Interface`) and served over HTTP
and gRPC at the same time. Both transports expose every method with the same
authorization and the same error reasons. The Go client
(`pkg/transport.NewClient`) returns an `api.Interface` for either transport.

## Methods

| Method | HTTP | gRPC method | Caller |
|--------|------|-------------|--------|
| Nodes.List | `GET /api/v1/nodes` | `NodesList` | admin, node |
| Nodes.Watch | `GET /api/v1/nodes?watch=true` | `NodesWatch` (stream) | admin, node |
| Nodes.Get | `GET /api/v1/nodes/{ref}` | `NodesGet` | admin, node |
| Nodes.Evict | `POST /api/v1/nodes/{ref}/evict` | `NodesEvict` | admin |
| Nodes.ListOverlayIPs | `GET /api/v1/nodes/{ref}/overlay-ips?free=true` | `NodesListOverlayIPs` | admin |
| Nodes.Register | `POST /api/v1/nodes/register` | `NodesRegister` | anyone with a join token |
| Nodes.SelfHeartbeat | `POST /api/v1/nodes/self/heartbeat` | `NodesSelfHeartbeat` | node |
| Nodes.SelfReportHardware | `PUT /api/v1/nodes/self/hardware` | `NodesSelfHardware` | node |
| Nodes.SelfRenewAPIKey | `POST /api/v1/nodes/self/api-key` | `NodesSelfRenewAPIKey` | node |
| Containers.List | `GET /api/v1/containers?node_id=&active=` | `ContainersList` | admin; node for its own `node_id` |
| Containers.Watch | `GET /api/v1/containers?watch=true&node_id=` | `ContainersWatch` (stream) | admin; node for its own `node_id` |
| Containers.Get | `GET /api/v1/containers/{id}` | `ContainersGet` | admin |
| Containers.Create | `POST /api/v1/containers` | `ContainersCreate` | admin |
| Containers.Delete | `DELETE /api/v1/containers/{id}` | `ContainersDelete` | admin |
| Containers.ListEvents | `GET /api/v1/containers/{id}/events` | `ContainersListEvents` | admin |
| Containers.ReportRuntime | `PUT /api/v1/containers/{id}/runtime` | `ContainersReportRuntime` | the node running it |
| Containers.AllocateOverlayIP | `POST /api/v1/containers/{id}/overlay-ip` | `ContainersAllocateOverlayIP` | the node running it |
| JoinTokens.List | `GET /api/v1/jointokens` | `JoinTokensList` | admin |
| JoinTokens.Create | `POST /api/v1/jointokens` | `JoinTokensCreate` | admin |

`{ref}` is a node ID (`node_...`) or a node name. `GET /healthz` returns `ok`
without authentication.

The gRPC service is `atellar.v1.Atellar`. Messages are the same JSON documents
as the HTTP bodies, sent with the `json` content subtype
(`application/grpc+json`), so no generated code is needed on either side.

## Authentication

Every call carries `Authorization: Bearer <token>` (HTTP header or gRPC
metadata). Two kinds of token exist:

- **Admin token**: `ADMIN_TOKEN` on the API server. Grants every operator call.
- **Node API key**: issued by `Nodes.Register`, valid for 90 days, renewed by
  the agent seven days before expiry. Only the SHA-256 hash is stored. Keys of
  evicted nodes are rejected.

When the server has no admin token (open mode), calls without a token are
treated as admin. Node keys keep working and keep their restrictions.

A missing or wrong token is `Unauthorized`. A valid token without access to the
method or object is `Forbidden`. The full caller by method matrix is enforced
by `pkg/apiserver/apiservertest.Cases()` and tested over both transports in
`pkg/transport/transport_test.go`.

## Errors

Every error carries a reason. Transports map it to their own codes and back
without loss.

| Reason | HTTP | gRPC |
|--------|------|------|
| BadRequest | 400 | InvalidArgument |
| Unauthorized | 401 | Unauthenticated |
| Forbidden | 403 | PermissionDenied |
| NotFound | 404 | NotFound |
| Conflict | 409 | AlreadyExists |
| Unavailable | 503 | Unavailable |
| Internal | 500 | Internal |

HTTP error bodies are `{"reason": "NotFound", "error": "<message>"}`.

## Watches

A watch first sends every matching object as an `ADDED` event, then one
`BOOKMARK` event with an empty object, then `ADDED`, `MODIFIED` or `DELETED`
for each change until the caller stops it.

Over HTTP the response has content type `application/json;stream=watch` and
one JSON event per line:

```
{"type":"ADDED","object":{"id":"node_1", ...}}
{"type":"BOOKMARK"}
{"type":"MODIFIED","object":{"id":"node_1", ...}}
```

A server side failure ends the stream with
`{"type":"ERROR","error":{"reason":"...","error":"..."}}`. Over gRPC each event
is one stream message and a failure ends the stream with a status.

Watches are not resumable. A client that loses its stream lists and watches
again; `pkg/client/informer` does this with exponential backoff.

## Container lifecycle events

The API server appends an event to a container's history on each transition:
`scheduled`, `pull_started`, `pull_finished`, `started`, `stopped`, `crashed`,
`backoff`, `restart`, `failed`, `removed`, `terminated`. `Containers.ListEvents`
returns them newest first.
