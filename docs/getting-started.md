# Getting started

This walks through a cluster with one API server and two nodes. All machines
must be Linux (amd64 or arm64) and reach each other on a private network.

## Requirements

| Machine | Needs |
|---------|-------|
| API server | PostgreSQL reachable from it (the test suite runs against 16) |
| Every node | containerd running, `iproute2` (`ip`, `bridge`), root access, UDP 4789 open between nodes |

## 1. Install the binary

On every machine:

```bash
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/install.sh | sudo bash
```

This installs `/usr/local/bin/atellar` and the database migrations under
`/usr/share/atellar/migrations`. It does not start anything.

## 2. Start the API server

```bash
sudo atellar server install \
  --database-url "postgresql://atellar:secret@localhost:5432/atellar?sslmode=disable"
```

The command creates the cluster CA and the server certificate under
`/etc/atellar/pki`, writes `/etc/atellar/api.env`, installs and starts the
`atellar-server` systemd unit, and prints:

- the **CA hash** (`sha256:...`). Clients and nodes use it to fetch the
  cluster CA once and check that they reached the real API server.
- a generated **admin token**. Keep it; it is stored only in `api.env`.

The API server only speaks TLS. See [config.md](config.md) for every option.

## 3. Point the CLI at the cluster

On the machine you operate from:

```bash
atellar config set-cluster prod --address 10.0.0.10 --ca-cert-hash <CA_HASH> --token <ADMIN_TOKEN>
atellar config set-context prod --cluster prod
atellar config use-context prod
atellar nodes list
```

The CLI talks gRPC on port 9090 by default. Add `--transport http` to use the
HTTP API on port 8080 instead.

## 4. Join nodes

Create a join token:

```bash
atellar join-tokens create
```

Tokens are single use and expire after 24 hours unless you pass `--multi-use`
or `--ttl`. The command prints the full join command, CA hash included. On
each node:

```bash
sudo atellar node install
sudo atellar node join \
  --join-token <TOKEN> \
  --ca-cert-hash <CA_HASH> \
  --name node-1 \
  --private-ip 10.0.0.21 \
  --control-plane-address 10.0.0.10
```

`node join` fetches the cluster CA, refuses to continue if it does not match
the hash, registers the machine, receives its subnet and API key, writes
`/etc/atellar/agent.json` and starts the agent. `--private-ip` is the address
other nodes use for VXLAN traffic.

```bash
atellar nodes list
```

A node is `ready` as soon as it joins. The scheduler places containers on it
once its agent has reported CPU and memory, which happens when the agent starts.

## 5. Run a container

```bash
atellar containers create --image docker.io/library/nginx:1.27 --cpu 0.5 --memory 256
atellar containers list
atellar containers events <ID>
```

Containers on different nodes reach each other on their overlay addresses.
To remove one:

```bash
atellar containers delete <ID>
```

## Running the API server in Docker

The repository includes a `Dockerfile` and `docker-compose.yml` that start
PostgreSQL and the API server with the admin token `dev-admin-token`. The
server creates its CA in the `pki` volume on first start and logs the CA hash:

```bash
docker compose up --build
docker compose logs server | grep "cluster CA"
atellar config set-cluster local --address localhost --ca-cert-hash <CA_HASH> --token dev-admin-token
```

## Upgrading nodes joined before TLS

Agent configurations written before the API server spoke TLS have no CA, and
the agent refuses to start with them. Run once on each such node:

```bash
sudo atellar node trust-ca --ca-cert-hash <CA_HASH>
```

Clusters in `~/.atellar/config` need `atellar config set-cluster` again with
`--ca-cert-hash`.

## Removing Atellar

```bash
atellar nodes evict node-1                     # from the operator machine
curl -fsSL https://github.com/hasirciogluhq/atellar/releases/latest/download/uninstall.sh | sudo bash
```

The uninstall script stops the units, removes local containers, the overlay
links, the binary, configuration and logs.
