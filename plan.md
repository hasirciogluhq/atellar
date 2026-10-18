# Atellar Architecture

Atellar hosts containers across many clusters. What we build: veth pairs, eBPF programs, a WireGuard backbone, internet egress, DNS, ingress and the control components. No Linux bridge, no VXLAN/Geneve devices, no third-party overlay. The current code is only a reference: no backward compatibility, it can be changed or removed freely.

Principles: Go first, least code for the best result, no needless abstraction, reuse code within the project and well-known libraries instead of writing duplicate code. In the data plane, use what the kernel already does (VRF routing, conntrack, nftables) instead of re-implementing it in eBPF.

# Namespaces

The **namespace** is Atellar's core scoping object, same idea as a Kubernetes namespace (not Kubernetes, not a Linux network namespace). Every object except cluster-wide ones (Node, Cluster, Namespace, roles) lives in exactly one namespace. How namespaces are used is up to the operator (per team, per customer, per environment, ...); Atellar has no tenant concept.

- **Namespaces are global, not per cluster.** Defined once in the Hub; namespace `A` in cluster 1 and namespace `A` in cluster 2 are the same namespace with the same type, CIDRs and ID.
- Network type, fixed at creation and **never changeable**: **Default** or **Private**.
- **Default**: all Default namespaces allocate from one global pool. An IP is unique across every Default namespace, node and cluster.
- **Private**: own CIDR(s). Its IPs may overlap with any other namespace, Private or Default (two Private namespaces can both use `10.12.44.3`, even on the same node).
- **Namespace ID**: users use the name (string). The Hub also assigns a `uint32` ID, globally unique, never reused, `0` reserved. The data plane uses the ID (map keys, packet header).
- Node-local identifiers (netns, ifindex, veth names, cgroup ids) **never leave the node**.

## IP uniqueness

| | same namespace | other Default namespace | other Private namespace |
|---|---|---|---|
| IP in a Default namespace | unique (all clusters) | unique (all clusters) | may overlap |
| IP in a Private namespace | unique (all clusters) | may overlap | may overlap |

## Reachability

| From \ To | same namespace | other Default namespace | other Private namespace | internet |
|---|---|---|---|---|
| Default | allowed | allowed by default, restricted by NetworkPolicy | **never** | allowed (namespace setting) |
| Private | allowed | **never** | **never** | allowed (namespace setting) |

Private namespaces are isolated by construction (own routing domain, see Data Plane), so NetworkPolicy applies to Default namespaces only. Ingress traffic (from the internet through the ingress) can reach both types.

## IP allocation

Uniqueness spans clusters, so the Hub owns the address space and hands each cluster address blocks per namespace (Default blocks from the global pool, Private blocks from the namespace's CIDRs). The Manager allocates container IPs from its blocks and asks for a new block when one runs low; it keeps allocating while the Hub is down until its blocks run out. A freed IP is not reused for 10 minutes.

## Dual stack

Every namespace has an IPv4 and an IPv6 CIDR; every container gets one address of each. Neither family can be disabled.

# Components

A cluster is not a process and not a set of special nodes. It is only a logical name for the nodes connected to the same Manager.

| Component | Command | Runs where | Role |
|-----------|---------|------------|------|
| Hub | `atellar hub run` | Outside every cluster, never on a node | Multi-cluster brain: placement, namespaces, IP blocks, identity, authz, global objects, API for CLI/WebUI |
| Manager | `atellar manager run` | Outside the cluster, never on a node | Brain of one cluster: scheduling, node lifecycle, path computation, route/endpoint sync |
| Agent | `atellar agent run` | Every node | Worker: runs containers, eBPF data plane, WireGuard, health checks, logs, metrics |
| DNS | `atellar dns run` | System DaemonSet on every node | Namespace-aware resolver fed by the Manager |
| Ingress | `atellar ingress run` | System DaemonSet on ingress nodes | HTTPS entry point, Let's Encrypt, routes to any cluster |
| WebUI | separate app (`web/`) | Anywhere | Admin UI, talks only to the Hub API |
| CLI | `atellar <verb>` | Operator machine | The one and only CLI |

One `atellar` Go binary covers every role (subcommands under `cmd/atellar`). Hub and Manager run as service processes in production; Docker is only for the local test suite.

Rules:

- **Every node is a worker.** No master node, no promotion, no Hub/Manager on a node.
- **The brain is never a node.** A Manager host does not run `atellar agent` and cannot run workloads.
- Nodes only talk to their Manager. Managers talk to the Hub. Nodes never talk to the Hub.

SOC: Hub decides *which cluster*, Manager decides *which node* and everything inside the cluster.

## State and high availability

| | Hub | Manager |
|---|---|---|
| Store | PostgreSQL (required) | PostgreSQL, or memory |
| Replicas | 1 (default) or N behind any load balancer, all active (stateless processes) | 1 (default) or N with Postgres; one leader via a lease in Postgres, standbys reject streams with the leader's address |
| Memory store | — | exactly 1 replica, rebuilt on restart |

- Neither process keeps long-lived local state (no local files, nothing that must survive a restart).
- Agents are the source of truth for what runs on their node and re-report full state on reconnect, so a Manager on the memory store recovers its cluster, even while the Hub is down.
- Default setup is one Hub + one Manager per cluster with one command each; HA (e.g. 3 replicas + a cloud load balancer) is the admin's choice and needs no extra Atellar component.

## Failure behaviour

| Down | Keeps working | Pauses |
|------|---------------|--------|
| Manager | All traffic, DNS (last known records), ingress, local path failover, running containers | Scheduling, new containers, route/endpoint propagation |
| Hub | Everything inside each cluster, Manager restarts | New namespaces, new IP blocks, cross-cluster changes, API/WebUI, joins |

Target: Manager restart + full cluster resync < 30s. The data plane has no timeout.

## Transport

gRPC everywhere, one schema: services and messages are defined in protobuf (`api/proto/`, generated with `buf`). No separate REST/JSON API.

| Link | Protocol | Shape |
|------|----------|-------|
| Hub <-> Manager | gRPC | one bidirectional stream (Manager dials Hub) |
| Manager <-> Agent | gRPC | one bidirectional stream (Agent dials Manager) |
| Manager <-> DNS, Ingress, other system workloads | gRPC | one bidirectional stream (workload dials Manager with its workload API key) |
| CLI -> Hub | gRPC | unary calls + server-streaming watches |
| WebUI -> Hub | Connect (gRPC-compatible, from the same protobuf) | unary calls + server-streaming watches |

- **Bidirectional stream**: the dialing side opens one long-lived stream and both sides push messages on it: commands and desired state down, reports, heartbeats and events up. On (re)connect the parent sends a full snapshot, then only deltas; each message carries a revision so the child can tell where it stands.
- **Workloads**: a system workload (DNS, ingress) opens its stream to the Manager of its cluster, sends a subscribe message naming what it needs (DNS: records; ingress: routes, backends, certificates), receives a snapshot, then changes. Same mechanism as agents, just a narrower subscription and narrower permissions. Normal user containers have no Manager connection.
- **WebUI**: browsers cannot speak native gRPC. The Hub serves gRPC, gRPC-Web and Connect on the same port from the same handlers (`connectrpc.com/connect`), and the WebUI uses the generated TypeScript client (`@connectrpc/connect-web`). Browsers also cannot run bidirectional streams, so the WebUI uses unary calls and server-streaming watches; nothing else needs bidi from the browser. Under the hood this is HTTP, but there is no hand-written HTTP API.
- All links use TLS with the pinned Atellar CA and bearer API keys in metadata.

## External integrations

Atellar is the runtime; other software (a hosting panel, billing, CI) drives it through the Hub API. That is the main way containers get created, not manual CLI use.

- The Hub's public API is the same gRPC services the CLI and WebUI use. Thanks to Connect, every method is reachable three ways on one port:
  - **gRPC**: generated clients for any language (`buf generate`: Go, TypeScript, Python, Java, ...)
  - **Connect/JSON**: plain HTTP `POST /atellar.v1.<Service>/<Method>` with a JSON body, works with `curl` or any HTTP library, no gRPC tooling needed
  - **gRPC-Web**: browsers
- Server-streaming watches (e.g. container status changes) work over all three, so an integration can follow what happens instead of polling.
- An integration authenticates as an **Automation** identity with an API key, limited by role bindings (e.g. may create/delete namespaces and manage workloads, may not touch nodes or clusters).
- `Create` calls accept an optional `request_id`; a retried call with the same ID returns the original result instead of creating a duplicate.
- We publish an official Go client package (`pkg/client`) on top of the generated code; other languages use the generated stubs or plain JSON.

# API, storage and objects

No YAML manifests and no `apply -f`. The API is typed: per kind `List`, `Get`, `Watch`, `Create`, `Update`, `Delete` as gRPC methods with typed protobuf messages; the CLI maps flags to those requests (`atellar deployments create --image ... --replicas 3`). It is still declarative where it matters: the database row holds the desired state (e.g. `replicas = 3`) and controllers reconcile towards it.

Storage: PostgreSQL behind one Go storage interface, sqlc-generated queries, SQL migrations applied at start; an in-memory implementation of the same interface for tests and the memory-backed Manager; one conformance suite both must pass.

- One table per kind with typed columns; Postgres enums for statuses.
- IDs: prefixed text IDs (`node_...`, `ctr_...`, `dep_...`), names unique per namespace.
- `labels JSONB` (GIN index, used by selectors) and `annotations JSONB` (free-form notes for tools and humans, never interpreted; one column, keeps labels clean) on every user-facing table.
- Optimistic concurrency: `resource_version BIGINT`, updated on every write; `Update` with a stale version returns `ErrConflict`.
- Every write publishes on the event broker (`pkg/events`), which feeds watches.

| Kind (table) | Lives in | Purpose |
|------|---------|---------|
| `namespaces` | Hub | name, ID, type, CIDRs, egress setting, runtime class |
| `clusters`, `nodes` | Hub (inventory), Manager (runtime) | labels, addresses, status, hardware |
| `containers` | Hub (spec), Manager (runtime) | one managed container |
| `deployments`, `daemonsets`, `jobs`, `cronjobs` | Hub | workload specs; Manager creates their `containers` |
| `services`, `ingresses`, `network_policies` | Hub | networking |
| `configs`, `secrets` | Hub | data for containers (secret values encrypted) |
| `users`, `api_keys`, `roles`, `role_bindings` | Hub | identity and authorization |
| `join_tokens` | Hub | node and Manager admission |
| `ip_blocks` | Hub (blocks), Manager (allocations) | IPAM |
| `events` | Manager, forwarded to Hub | what happened to an object |

Placement: workload rows carry a cluster selector (labels such as `region=eu`, `tier=3`) plus resources; the Hub picks the best matching cluster and sets `cluster_id`. A Deployment runs in one cluster. A DaemonSet may select several clusters.

# Workloads

There are no pods: the unit is the container.

- **Container**: one container, restarted per `restart_policy` (`always`, `on-failure`, `no`) with exponential backoff.
- **Deployment**: desired replica count; scaling changes `replicas`, never creates loose containers. Rolling update (`maxSurge`, `maxUnavailable`), gated by readiness. Keeps the last 10 revisions; `atellar rollout undo` rolls back. Replica names: `<deployment>-<5 random chars>`.
- **DaemonSet**: one container per node matching its cluster and node selectors. Never rescheduled elsewhere.
- **Job**: runs to completion, `backoffLimit` retries, `parallelism`. **CronJob**: creates Jobs on a cron schedule, `concurrencyPolicy` (`Allow`, `Forbid`, `Replace`). Run by the Manager leader.
- **Health checks** (all workload kinds): `startup`, `liveness`, `readiness` probes of type `exec`, `http`, `tcp`, with `initialDelay`, `period`, `timeout`, `failureThreshold`. The agent runs them inside the container's netns, so overlapping IPs never matter. Liveness failure restarts; readiness gates Service backends and rollouts.
- **Resources**: `cpu`, `memory` requests (scheduling) and limits (enforced, cgroup v2), `pids`, optional `bandwidth`.

## Node failure

The goal: a lost node's work moves elsewhere, and the same replica never runs twice.

1. A node without heartbeats is `NotReady` after 30s. Nothing moves yet.
2. Before declaring it lost, the Manager asks the node's peers whether they still reach it over the backbone. A node that peers still reach is only cut off from the Manager and stays `NotReady`.
3. A node unreachable by the Manager **and** its peers for `nodeLostAfter` (default 5m) becomes `Lost`. Deployment replicas, Job containers and Containers with `failurePolicy: Reschedule` (the default) are recreated elsewhere with new names and IPs. DaemonSet containers and Containers with `failurePolicy: Stay` are never moved.
4. **Self-fencing** on the node side: an agent that reaches neither its Manager nor a majority of its peers for `fenceAfter` (default 4m, always < `nodeLostAfter`) stops its reschedulable containers. A Manager outage alone never stops anything (peers are still reachable), and a single-node cluster never fences.
5. When a lost node returns it reports its containers; those that were replaced are deleted, the rest are adopted, and the node becomes `Ready`. Their old IPs are only reused after the 10 minute cooldown.

# Networking

## Backbone

Nodes live in different networks/locations (e.g. İzmir and İstanbul). Most have a public IP, some don't (internal nodes, VMs on a bare-metal host behind NAT). Some share a private underlay. Nothing is configured: a node is installed and discovery finds its paths.

### Node addressing

- Every node gets one **backbone IP** (IPv4 `/32`, Hub-allocated, from `100.64.0.0/10`) on a dummy interface. The backbone is internal; WireGuard endpoints themselves may be IPv4 or IPv6.
- All node-to-node packets use backbone IPs as outer source/destination. The path is only a kernel route for the peer's backbone IP.
- WireGuard `AllowedIPs` contain backbone IPs only, never container IPs, so overlapping IPs never confuse WireGuard.

### Segments

A **segment** is a set of nodes that reach each other directly over a private underlay (e.g. `{A,B,C}`, `{D,E,F}`, `{G,H,I}`). Segments are independent of clusters, never configured, found by discovery. Any node can join from anywhere.

### Paths

Per node pair, the first path that works wins:

1. **underlay**: same segment → `route <peer backbone ip> via <peer underlay ip>`
2. **wireguard direct**: at least one side reachable (public IP, or a NAT mapping opened by hole punching)
3. **wireguard relay**: via a relay node when neither side is reachable

No dedicated relay server: any node with a public IP can relay; the Manager picks one per pair (lowest latency, spread). WireGuard is hop by hop, so a relay decrypts and re-encrypts; operators can forbid nodes as relays. A segment marked **untrusted** uses WireGuard even inside the segment.

### Discovery

Automatic on join:

1. Agent reports interface addresses and its WireGuard public key to the Manager.
2. A public node sees a NAT'd node's mapped endpoint on the first handshake (`wg show endpoints`) and reports it. NAT'd nodes keep mappings open with `PersistentKeepalive`.
3. Manager sends each agent the addresses/endpoints of the peers it needs.
4. Agents probe peers' private addresses (UDP on the encap port) and hole-punch NAT'd peers from both sides at once.
5. Manager builds segments, picks a path per pair (underlay > direct > relay), pushes it. Agents program WireGuard peers and backbone routes.
6. Segments and paths go to the Hub; WebUI/CLI show them with health and allow overrides (pin path, choose/forbid relays, untrusted segment, disable peer). Overrides live in the Hub.
7. Agents keep probing; a failing path falls back locally and returns after a few good probes.

A node with no public IP and no shared underlay needs at least one public node in its cluster. Cross-cluster pairs (only pairs sharing a namespace) follow the same flow, coordinated by the Hub.

## Data plane

### Platform support

Every current mainstream distro with its stock kernel: Ubuntu 22.04+, Debian 12+, RHEL/Rocky/Alma 9+, Fedora, Alpine, Arch, Gentoo, ...

- Minimum kernel **5.10** (`bpf_redirect_peer`, `bpf_redirect_neigh`, in-tree WireGuard, VRF, cgroup v2 socket hooks). Features from newer kernels (TCX, user namespaces with idmapped mounts) are used when present.
- Programs read packet/socket data only → no CO-RE/BTF dependency, one prebuilt object for every kernel. Built with clang, embedded with `bpf2go` (`cilium/ebpf`).
- Attach with TCX on ≥ 6.6, `clsact` otherwise.

### Code layout

- `bpf/`: eBPF C sources and headers
- `pkg/network/datapath/`: loader, maps, attach (`bpf2go` output)
- `pkg/network/backbone/`: WireGuard, backbone routes, discovery
- `pkg/network/service/`, `pkg/network/policy/`: service LB maps, policy compiler

### Routing domains (VRF)

A **routing domain** is one VRF device per node with its own table:

- **Default domain**: one VRF shared by all Default namespaces (their IPs never overlap, cross-namespace traffic is allowed by default and filtered by policy).
- **Private domain**: one VRF per Private namespace present on the node. No routes to anything else exist, which is what makes Private namespaces isolated.

Each VRF contains:

- the host side of each container veth (`atl<id>`), with `/32` and `/128` routes to it
- one `atl-tx<domain>` dummy device; routes for remote prefixes point at it
- a default route for internet egress (if the namespace allows it)
- the DNS address `169.254.1.53` / `fd00:a7e1::53` on the VRF device

The kernel does all routing inside a domain. eBPF only encapsulates what leaves the node and decapsulates what arrives.

### Container interface

- veth pair: `eth0` in the container netns, `atl<id>` in the host netns, no bridge, MTU **1329** (worst case need 1384, rest is headroom).
- IPv4: `default via 169.254.1.1 dev eth0 onlink`; IPv6: `default via fe80::1 dev eth0`. Static neighbor entries for both gateways; every host veth uses the fixed MAC `02:a7:e1:00:00:01`.
- Container MAC derived from its IPv4 address: `02:a7` + 4 bytes of IPv4 (`10.12.44.3` → `02:a7:0a:0c:2c:03`). Every container has an IPv4 address (dual stack is mandatory), so the same scheme covers IPv6 too. The agent writes the neighbor entries for both families from the IP alone; no ARP or NDP is needed between host and container.
- Container sysctls: `accept_ra=0`, `autoconf=0`, addresses added with `nodad`.

### Anti-spoofing

`from_container` drops, for both families:

- source IP not equal to the container's assigned IPv4/IPv6 (link-local `fe80::` derived from the MAC is allowed for NDP)
- source MAC not equal to the container's MAC
- IPv6 Router Advertisements, Redirects, and NDP messages for addresses the container does not own
- DHCP server traffic

### Encapsulation

```
outer IPv4 (backbone src -> backbone dst) | UDP (dst 7472) | atellar header | inner IPv4/IPv6 packet
```

| Field | Size | Value |
|-------|------|-------|
| magic | 2 | `0xA7E1` (fixed forever) |
| version | 1 | `1` |
| flags | 1 | bit 0: inner is IPv6 |
| namespace_id | 4 | destination namespace ID |

UDP source port = hash of the inner flow. Overhead 36 bytes.

### Maps

| Map | Key | Value | Written by |
|-----|-----|-------|------------|
| `endpoints_local` | host veth `ifindex` | `{namespace_id, ipv4, ipv6, mac}` | Agent |
| `domains` | `atl-tx` `ifindex` | `domain_id` | Agent |
| `ns_vrf` | `namespace_id` | VRF `ifindex` (all Default namespaces → Default VRF) | Agent |
| `remote` (LPM trie) | `{domain_id, prefix}` | `{node backbone ip, namespace_id}` | Agent, from Manager |
| `nodes` | backbone ip | known node, validates decap source | Agent |
| `flows` (ring buffer) | — | flow/drop records | eBPF |

`domain_id` = namespace ID for Private domains, reserved `1` for the Default domain.

### Packet path

- `from_container` (host veth ingress): anti-spoofing, bandwidth (EDT timestamp), flow record, `TC_ACT_OK`. The kernel routes in the VRF: local container → its veth; remote → `atl-tx`; else internet egress.
- `to_remote` (`atl-tx` egress): `remote[{domain, dst}]` → push outer headers → `bpf_fib_lookup` (main table) + `bpf_redirect_neigh`. Underlay/WireGuard/relay stay plain kernel routes.
- `from_backbone` (WireGuard/underlay ingress): check port, magic, version, outer src in `nodes`; strip outer headers; `bpf_fib_lookup` in `ns_vrf[namespace_id]` → `bpf_redirect_neigh` to the veth the kernel returns.

Host processes (ingress, DNS, health checks) reach any domain by binding a socket to its VRF device; the same kernel routes and `atl-tx` encapsulation apply.

### Internet egress

- VRF default route leaks to the host uplink.
- nftables: one `ct zone` per VRF + `masquerade` (IPv4) / NAT66 masquerade (IPv6), `ct mark` = zone, so overlapping IPs never collide in conntrack.
- Reply: `meta mark set ct mark` → `ip rule fwmark <zone> lookup <VRF table>`.

### Bandwidth and fairness

Per-container egress limit with EDT (eBPF sets `skb->tstamp`, `fq` qdisc paces). CPU, memory, IO and pids via cgroup v2 limits keep neighbors from starving each other.

### Map sync

- Local: on container create/delete the agent sets up the veth, routes, neighbors, `endpoints_local`, then reports.
- In-cluster: Manager pushes `{domain, prefix} → node` to nodes hosting the same domain.
- Cross-cluster: Manager → Hub → other Managers with the same namespace → their nodes.
- Reconnect: full set, then deltas.

## Services

A **Service** gives a stable virtual IP pair (**ServiceIP**, IPv4 + IPv6) and a DNS name to the ready containers matching its selector in the same namespace, across all clusters.

- ServiceIPs come from the namespace's domain: Default Services from a global Default service pool (unique), Private Services from the namespace's own service CIDR (may overlap with other namespaces).
- Load balancing happens at the socket, not per packet: eBPF cgroup hooks (`connect4/6`, `sendmsg4/6`, `recvmsg4/6`, `getpeername4/6`) rewrite the ServiceIP to a backend address when the application connects. Packets on the wire always carry real container IPs, so routing, policy and encapsulation stay unchanged and there is no per-packet NAT.
- Key `{domain_id, serviceIP, port, proto}`; the domain comes from the caller's cgroup (container) or bound VRF (host process).
- Backend choice: random among ready backends, same-cluster backends first, other clusters as fallback.
- A container may reach itself through its own Service (hairpin); with socket LB this needs no special handling, so it is allowed.

## DNS

`atellar dns` is our own Go DNS server (`miekg/dns`), a system DaemonSet with host networking on every node.

- Listens on `169.254.1.53` / `fd00:a7e1::53` (present in every VRF, `udp/tcp_l3mdev_accept=1`). Containers get it in `resolv.conf`.
- Identifies the asking container by the arriving interface (read-only access to the agent's pinned `endpoints_local` map), so answers are namespace-aware.
- Connects to the Manager with its workload identity and watches records (containers, Services); every change is pushed, no polling. Keeps serving the last records if the Manager is down.
- Names (defaults; domain and templates configurable by flags/env, e.g. `ATELLAR_DNS_DOMAIN`):
  - same namespace: `<name>.atellar.internal`
  - other namespace: `<name>.<namespace>.atellar.internal` (Default namespaces only; Private gets `NXDOMAIN`)
  - `<name>` is a container (including generated replica names) or a Service; names are unique per namespace across both.
- Other names are forwarded to the node's upstream resolvers.

## Ingress

`atellar ingress` is our own Go HTTPS proxy, a system DaemonSet with host networking on nodes labeled `atellar.io/ingress=true`. No public ports for containers: everything public goes through ingress.

- Ingress objects (`host`, `path` → Service + port) are global like namespaces, so every ingress instance in every cluster knows every route. Traffic may enter any ingress node in any cluster and is proxied to a backend wherever it runs; a remote backend is reached through the backbone like any other traffic.
- The proxy resolves Service backends itself (pushed by its Manager) and connects straight to container IPs by binding to the target namespace's VRF. Works for Default and Private namespaces.
- TLS: automatic Let's Encrypt (ACME HTTP-01 and TLS-ALPN-01). One instance per hostname (lease) runs the ACME order; challenge tokens are distributed so any instance can answer. Certificates are stored as Secrets in the system namespace.
- HTTP → HTTPS redirect, HTTP/2, WebSocket, configurable timeouts.

## NetworkPolicy

Applies only to Default namespaces (the API rejects it in Private ones). Default: everything between Default namespaces is allowed. A policy selects containers by labels and lists allowed ingress/egress peers (namespace selector, label selector, CIDR) and ports.

- The Manager **compiles** policies into flat rules: selectors resolved to IP sets per node.
- Agents apply them as nftables sets in the Default VRF's forward/output path, enforced at the source node, stateful via kernel conntrack (replies are allowed automatically). A policy change only swaps set contents.

## Flow visibility

- eBPF writes flow start/end and drop records (with reason: policy, spoof, no route, ...) to a ring buffer.
- The agent aggregates them per flow (bytes, packets, duration) and exports every 10s to the Manager, which forwards to the Hub.
- `atellar flows` and the WebUI query and stream them by namespace, container, Service or node. Agents also expose them as Prometheus counters.

# Config, Secrets and mounts

- **Config**: key/value or files, plain text.
- **Secret**: same shape, encrypted at rest in the Hub (AES-256-GCM envelope encryption, key from Hub config; rotation re-encrypts). Sent to a node only when a container there uses it, kept in memory/tmpfs on the node, never on disk.
- A container can mount a whole Config/Secret as files, single keys as files, and/or set env vars from whole objects or single keys, all at once.
- Mounts are only Config/Secret files. `hostPath` mounts are allowed only in the system namespace (log shippers, node tools). No persistent volumes yet.

# Observability

- **Logs**: the agent writes stdout/stderr to `/var/log/atellar/containers/<namespace>/<name>/<instance>.log` (JSON lines, rotated 10MB × 5). `atellar logs [-f]` streams through Manager → agent. Shipping elsewhere (OpenObserve, Loki, ...) is a DaemonSet mounting `/var/log/atellar` via `hostPath`; Atellar ships nothing itself.
- **Metrics**: every component exposes `/metrics` (Prometheus). The agent covers the node and every container (cgroup v2 CPU/memory/IO/pids, per-veth traffic from eBPF, probe results). Manager and Hub expose scheduling, sync and API metrics.
- **Events**: `Event` objects (`type`, `reason`, `object`, `message`, `count`), kept for 1h in the Manager and forwarded to the Hub. `atellar events [-w]`.

# Security

## Identity

TLS everywhere with a pinned CA (kubeadm-style `sha256:` pin) and bearer API keys stored only as SHA-256 hashes, each with an expiry and a self-renew call.

- The Hub owns the single Atellar CA. It issues the Hub's and every Manager's server certificate; agents, DNS and ingress pin the same CA hash when they first contact their Manager.
- Every identity authenticates with a bearer API key:

| Identity | Issued by | Stored in |
|----------|-----------|-----------|
| User, Automation | Hub | `api_keys` (Hub) |
| Manager | Hub, on cluster registration | `api_keys` (Hub) |
| Node / Agent | Hub, on node registration (via the Manager) | `api_keys` (Hub) |
| Workload (DNS, ingress, other system DaemonSets) | Manager | Manager store |

- The admin token from config stays as the bootstrap / break-glass credential (mandatory).
- The Manager trusts the Hub. The Hub trusts a Manager's key but never identities inside a cluster. A container that wants to act on the cluster asks its Manager.

## Authorization

Actions are defined in code; each action is granted by one or more scopes (`map[Action][]Scope`).

- Node, Manager and workload principals have fixed scope sets in code.
- Users and automations get scopes from the database:
  - `roles`: `id`, `name`, `scopes TEXT[]`; built-in roles (`admin`, `viewer`, `deployer`) seeded by a migration, custom roles created through the API.
  - `role_bindings`: `subject_type`, `subject_id`, `role_id`, `namespace_id` (NULL = all namespaces).
- `Authorize(ctx, action, namespace)`: allowed if any binding of the principal that covers that namespace grants a scope listed for the action. Deny by default.
- Every write is recorded by the Hub in an `audit_log` table.

Identity != what identity could do!

## Joining

Join tokens are stored hashed, single or multi use, with expiry. Registration consumes the token, creates the node and issues its API key in one transaction.

1. Admin creates a join token in the Hub; the token carries a `cluster_id` (or a `kind = manager` for registering a new cluster's Manager).
2. `atellar node join --manager <addr> --token <t> --ca-cert-hash sha256:...`: the agent pins the CA and calls `Register` on its Manager.
3. The Manager forwards `Register` to the Hub, which runs the transaction (validate, consume token, create node, issue API key) and returns the response through the Manager. Joins need the Hub online (rare, acceptable).

## Rotation

- API keys are short-lived (30 days) and renewed automatically at 2/3 of their lifetime through a self-renew call; server certificates are re-issued before expiry.
- WireGuard static keys rotate every 30 days with overlap (session keys already rotate every 2 minutes inside WireGuard).
- Admins can revoke or rotate any key, certificate or the secret encryption key from the Hub.

## Container isolation

Defaults for every container:

- cgroup v2 limits, `no-new-privileges`, minimal capabilities, default seccomp profile (blocks dangerous syscalls), AppArmor/SELinux default profile where the distro has one
- user namespaces (container root ≠ host root) when the kernel supports them
- per-namespace `runtimeClass`: `runc` (default) or **gVisor** (`runsc`, a user-space kernel that sandboxes untrusted code; recommended for Private namespaces of untrusted customers)

# Upgrades

- eBPF maps are pinned under `/sys/fs/bpf/atellar`; an agent restart or upgrade keeps maps and running containers, and programs are swapped atomically (TCX link update / `tc replace`). Map layouts carry a version; a new agent migrates them on start.
- containerd keeps containers running while the agent restarts; the new agent re-adopts them by labels.
- Version skew: Manager and Agent may be one minor version behind the Hub. Upgrade order: Hub → Managers → Agents (rolling, node by node).

# WebUI

Separate app in `web/`, not embedded in the Hub; uses only the Hub API. Network settings in full detail: identity, keys, nodes, clusters, namespaces, segments/paths/WireGuard (auto-discovered, overridable), flows, policies, labels/annotations, all cluster-level settings. Runtime info is streamed live from the Hub.

tanstack-start, file router, shadcn (base: mira). Everything native shadcn; no custom components except the layout and a few truly special ones, otherwise wrapper components that group several shadcn components into a widget.

# Out of scope (for now)

- Persistent volumes and object storage (later: S3 and Ceph)
- Autoscaling
- Load balancing algorithms beyond random + locality, session affinity
- Multi-cluster Deployments (one Deployment = one cluster)
