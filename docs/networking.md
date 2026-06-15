# Overlay & Container Networking

Linux agent networking model: per-node bridge (`atellar0`) + veth pairs into per-container network namespaces.

## IPAM layout

| Layer | Source | Example |
|-------|--------|---------|
| Cluster CIDR | `CLUSTER_OVERLAY_CIDR` (API) | `10.0.0.0/8` |
| Node subnet | `/24` per node (`NODE_SUBNET_PREFIX_LEN`) | `10.0.2.0/24` |
| Node gateway IP | First host in subnet → `overlay_ip` in agent config | `10.0.2.1` |
| Container IP | `overlay_ip_pool` on that node | `10.0.2.50` |

On join/register, the control plane assigns `overlay_ip` + `overlay_subnet` and writes them to `/etc/atellar/agent.json`. The overlay manager applies the node IP on `atellar0`; container netns setup uses the same values as the default gateway.

Current implementation status:

- Creates and reconciles the local bridge address.
- Creates per-container netns/veth wiring on the scheduled node.
- Receives peer node/container events and runs route reconciliation.
- Does **not** create VXLAN, WireGuard, or any other node-to-node tunnel.
- Does **not yet** program `private_ip` as the cross-node next hop. Multi-node overlay traffic is experimental until this route manager is completed.

For multi-node experiments, node `private_ip` values should still be mutually reachable because that is the intended data-plane address. If nodes are not on the same routable underlay, create WireGuard yourself and join each node with its WireGuard IP as `private_ip`.

## Per-container netns setup

`internal/agent/netns/setup_linux.go` (called from `runtime.Manager.runPipeline`):

1. Enable `net.ipv4.ip_forward` on the host
2. Teardown any stale netns/veth from a previous failed attempt
3. Create netns + veth pair (host leg → bridge, peer leg → netns)
4. Assign container address: `{container_overlay_ip}/{subnet_prefix}` (prefix from `overlay_subnet`, default `/32` if missing)
5. Default route: `via {node_overlay_ip} dev {veth}` — adds `onlink` only when prefix is `/32`

**Required config for reliable routing:**

```json
{
  "overlay_ip": "10.0.2.1",
  "overlay_subnet": "10.0.2.0/24",
  "bridge_name": "atellar0"
}
```

`runtime.NewManager` passes `overlay_ip` and `overlay_subnet` into `netns.Setup` as `NodeOverlayIP` and `NodeOverlaySubnet`. If both are empty and the bridge has no IPv4 address, setup fails with:

```
node overlay gateway ip is not configured on atellar0
```

### Common failure: `Nexthop has invalid gateway`

| Cause | Fix |
|-------|-----|
| `atellar0` has no IPv4 | Ensure agent overlay reconcile ran; check `ip addr show atellar0` |
| `overlay_ip` missing in `agent.json` | Re-join or `PATCH /api/v1/nodes/:id/overlay`; restart agent |
| Container `/32` + wrong gateway | Set `overlay_subnet` so container gets `/24` on same L2 segment as bridge |
| Stale veth/netns from failed run | `ip link del <veth>` + `ip netns del <ctr_id>`; restart agent |
| `snapshot "...": already exists` | Orphan containerd snapshot from partial create; agent purges on retry (`PurgeState`) — redeploy agent or `ctr -n atellar snapshots rm <id>` |
| `args must not be empty` (runc) | Deploy specified image only; agent must apply `oci.WithImageConfig` so ENTRYPOINT/CMD come from the image |

## Quick diagnostics

Run on the **scheduled node** (not the control plane):

```bash
# Node bridge + routes
ip addr show atellar0
ip route

# Agent config
cat /etc/atellar/agent.json | jq '{overlay_ip, overlay_subnet, bridge_name}'

# Container list (uses current atelctl context)
atelctl cluster containers list

# Per-container netns (replace ctr ID)
ip netns exec ctr_<id> ip addr
ip netns exec ctr_<id> ip route

# Agent logs (systemd unit)
journalctl -u atellar-agent.service -n 100 --no-pager
```

Expected healthy state on node `10.0.2.1/24`:

```
# ip addr show atellar0
inet 10.0.2.1/24 ...

# ip netns exec ctr_xxx ip addr
inet 10.0.2.50/24 dev vh...

# ip netns exec ctr_xxx ip route
default via 10.0.2.1 dev vh...
```

## Cleanup after failed container

```bash
sudo ip link del vh<hash> 2>/dev/null || true   # host veth name from ip link | grep vh
sudo ip netns del ctr_<container_id> 2>/dev/null || true
sudo systemctl restart atellar-agent.service
```

Delete the failed workload from the control plane, then redeploy.

## Troubleshooting playbooks

Use these prompts when debugging with logs/CLI output attached.

### 1. Container cannot reach gateway / bridge

```
netns kurulum sonrası container içinden gateway'e ping atılamıyor.
ip netns exec <id> ip route ve ip addr çıktıları şunlar: [...]
Bridge: atellar0, gateway: <node_overlay_ip>, container IP: <ip>/<prefix>
```

Check: veth enslaved to bridge, container address prefix matches node subnet, default route gateway equals `overlay_ip` from agent config.

### 2. Two containers on same node cannot reach each other

```
Aynı bridge'e bağlı iki container birbirine ping atamıyor.
ip netns exec ctr1 ping <ctr2_ip> → destination host unreachable
Her ikisi de atellar0'a bağlı veth pair'leri var.
```

Check: both containers in same `overlay_subnet`, bridge forwarding (`bridge link show`), no iptables DROP on `FORWARD`, `ip_forward=1`.

### 3. Cross-node overlay traffic fails

```
Node1 container (<ip1>) → Node2 container (<ip2>) ping atamıyor.
Each node has an overlay subnet and atellar0 is up.
Node private_ip values are expected to be mutually reachable, but the current agent does not yet use them as route next hops.
```

Check: node-to-node underlay reachability first (`ping <peer_private_ip>`), then check whether the current agent route state can resolve the remote container (`ip route get <remote_container_ip>`) and whether control-plane events were delivered (`container.scheduled`). If using WireGuard, verify the peer IP used at join is the WireGuard IP and AllowedIPs include the peer node/container ranges. If the route next hop is not the peer `private_ip`, this is a known current limitation rather than an operator setup issue.

### 4. Stale netns / veth after terminate

```
Container terminate sonrası ip netns list eski ID'leri gösteriyor.
Teardown çalışıyor ama ip netns del bazen:
"Cannot remove namespace file: device or resource busy"
```

Check: containerd task still holding netns (`ctr tasks list`), orphan veth (`ip link show master atellar0`), manual `ip link del` then `ip netns del`. Agent calls `Teardown` at start of `Setup` for idempotent retries.

### 5. Container cannot reach internet (NAT)

```
Container bridge/gateway ping OK, 8.8.8.8 unreachable.
ip_forward=1 set. iptables -L -n -v → [...]
```

Atellar does not configure NAT by default. For outbound internet from containers, add MASQUERADE on the node's uplink interface (example):

```bash
iptables -t nat -A POSTROUTING -s 10.0.0.0/8 ! -o atellar0 -j MASQUERADE
```

Adjust source CIDR and `-o` interface to match your deployment.

## Related code

| Path | Role |
|------|------|
| `internal/agent/netns/setup_linux.go` | veth, netns, default route |
| `internal/agent/runtime/manager.go` | pipeline; passes node overlay into netns |
| `internal/agent/overlay/` | bridge IP and peer route reconcile |
| `internal/cluster/ipam/` | control-plane subnet + container IP allocation |
| `docs/peer-events.md` | `node.*` / `container.scheduled` events |
