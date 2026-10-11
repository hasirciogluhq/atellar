# Networking

Every container gets an IPv4 address that is reachable from every other
container in the cluster. Atellar builds this with a VXLAN overlay, one
subnet per node and no per-container state outside the owning node.

## Address allocation

The API server owns all address allocation (`pkg/network/ipam`).

- `CLUSTER_OVERLAY_CIDR` (default `10.0.0.0/8`) is split into node subnets of
  `NODE_SUBNET_PREFIX_LEN` bits (default `/24`).
- `Nodes.Register` gives the joining node the next free subnet. A subnet of an
  evicted node is reused before a new one is carved.
- In each subnet the first host address (`.1` for a /24) is the node address,
  held by the node's bridge. The network address (`.0`) is the node's VTEP
  address. Every other host address goes into the node's container pool.
- `Containers.AllocateOverlayIP` takes an address from the pool of the node
  the container runs on. Termination returns it.
- Allocation runs inside a database transaction with an advisory lock, so
  concurrent joins never receive the same subnet.

## Devices on a node

| Device | Default name | Address |
|--------|--------------|---------|
| Bridge | `atellar0` | node address with the subnet prefix, e.g. `10.0.3.1/24` |
| VXLAN | `atellar-vx` | VTEP address `/32`, e.g. `10.0.3.0/32`; VNI 4242, UDP 4789, `nolearning` |
| veth pair per container | `vh<hash>` on the host, `vp<hash>` in the namespace | container address, default route via the node address |

The VXLAN device's MAC address is `02:41` followed by the four bytes of the
subnet address, so every node can compute any other node's MAC from its
subnet. Bridge, VXLAN and veth MTUs are 1450 by default, which leaves room for
the 50 byte VXLAN header on a 1500 byte underlay.

## Peer entries

For each other node that has a subnet and an underlay address, the agent
programs three entries on the VXLAN device:

```
ip route  replace <peer subnet> via <peer VTEP> dev atellar-vx onlink
ip neigh  replace <peer VTEP> lladdr <peer MAC> dev atellar-vx nud permanent
bridge fdb replace <peer MAC> dev atellar-vx dst <peer underlay IP>
```

The peer underlay IP is the peer's private IP, or its public IP when it has no
private one. Nodes that are `evicting`, `evicted` or `down`, or have no subnet
yet, get no entries. A `not_ready` node keeps its entries: its containers may
still be running while only its link to the API server is broken.

The agent reads the current entries with `ip -j` and `bridge -j`, computes the
difference against the node list and applies only the changes, in the order
FDB, neighbors, routes, so a new route never points at an unresolved next
hop. Entries that are already correct are not touched. The reconcile runs on every node list change and every 30 seconds, so manual
changes to these entries are repaired.

A packet from a container on node A to a container on node B:

1. Leaves the container through its veth to `atellar0` on A.
2. Matches the route for B's subnet, next hop B's VTEP on `atellar-vx`.
3. The permanent neighbor entry gives B's VXLAN MAC; the FDB entry for that
   MAC gives B's underlay address.
4. The kernel encapsulates it in UDP 4789 to B. B decapsulates it on its own
   `atellar-vx` and routes it to `atellar0` and the container's veth.

## Requirements and limits

- All nodes must reach each other's underlay address directly (same network
  or routed, no NAT between nodes).
- UDP 4789 must be open between nodes.
- `iproute2` must be installed. The agent enables `net.ipv4.ip_forward`.
- IPv4 only.
- No network policy or encryption on the overlay yet.

## Inspecting a node

```bash
ip -d link show atellar-vx
ip route show dev atellar-vx
ip neigh show dev atellar-vx
bridge fdb show dev atellar-vx
ip netns list
```
