# Node agent

`atellar agent run` is the node agent. systemd starts it as
`atellar-agent.service` after `atellar node join` has written
`/etc/atellar/agent.json`. It reads only that file (see
[config.md](config.md)).

## Loops

| Loop | Interval | Does |
|------|----------|------|
| Heartbeat | 5s (`heartbeat_interval`) | `Nodes.SelfHeartbeat`; keeps the node `ready` |
| Hardware | at start, then every 10m | Reports CPU cores, memory, disk, OS, kernel |
| Key renewal | hourly check | Renews the API key seven days before expiry and saves it to the config file |
| Node informer | watch | Keeps the node list for the overlay network |
| Overlay | on change and every 30s | Programs the VXLAN device, routes, neighbors and FDB ([networking.md](networking.md)) |
| Container informer | watch | Wakes the workload manager on any change to this node's containers |
| Workload manager | on wake and every 15s | Lists this node's containers and converges containerd to them |

Informers reconnect with exponential backoff (up to 30s) and re-list after a
reconnect, so an API server restart does not need an agent restart.

## Running a container

For a container in `pending` or `scheduled`, the workload manager:

1. Reports `pulling` and pulls the image through containerd (namespace
   `atellar`). An image already present locally is not pulled again.
2. Reports `creating` with the image digest.
3. Calls `Containers.AllocateOverlayIP` to get an address from the node's
   subnet.
4. Creates a network namespace at `/run/netns/<id>`, a veth pair to the
   bridge, the address and a default route via the node's bridge address.
5. Creates the containerd container with the image config, command, env and
   resource limits (CFS quota for `cpu_limit`, cgroup memory limit for
   `memory_limit_mib`) and starts its task. Output goes to
   `/var/log/atellar/<id>.log`.
6. Reports `running` with the task PID.

A failure in any step tears the namespace and containerd state down and
reports `backoff`, or `failed` when the restart policy is `no`.

Each container is handled by at most one goroutine at a time.

## Exits and restarts

The agent subscribes to containerd task exit events.

| Exit | Status | Restarted when policy is |
|------|--------|--------------------------|
| code 0 | `stopped` | `always` |
| code != 0 | `crashed` | `always`, `on-failure` |
| task gone while `running` | `crashed` | `always`, `on-failure` |

Restarts wait 10s, doubling with each restart up to 5 minutes. There is no
retry limit. The restart count is reported with every attempt and recorded as
a `restart` event.

## Deletion

`Containers.Delete` marks the container `removed`. The agent stops the task
(SIGTERM, SIGKILL after 10 seconds), deletes the container and snapshot,
removes the network namespace and reports `terminated`. The API server then
releases the overlay address.
