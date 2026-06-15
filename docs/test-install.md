# Test Install

Small smoke test for a node where release binaries are already installed.

## 1. Configure client context

Run on the machine where you will issue admin/client commands:

```bash
atelctl config set-cluster lab \
  --control-plane-address api.atellar.orb.local \
  --http-port 8080 \
  --grpc-port 9090
atelctl config set-context lab --cluster lab
atelctl config use-context lab
```

## 2. Create a join token

```bash
curl -X POST http://api.atellar.orb.local:8080/api/v1/nodes/join-tokens \
  -H "Content-Type: application/json" \
  -d '{"single_use": true}'
```

Save the returned `token`.

## 3. Join the node

Run on the node machine:

```bash
sudo ateladm node install --auto-join \
  --join-token <TOKEN> \
  --name vm3 \
  --public-ip 192.168.139.119 \
  --private-ip 192.168.139.119
```

If the node does not have `~/.atellar/config`, pass endpoint flags explicitly:

```bash
sudo ateladm node install --auto-join \
  --join-token <TOKEN> \
  --name vm3 \
  --public-ip 192.168.139.119 \
  --private-ip 192.168.139.119 \
  --control-plane-address api.atellar.orb.local \
  --http-port 8080 \
  --grpc-port 9090
```

## 4. Verify

```bash
atelctl cluster nodes list
journalctl -u atellar-agent.service -f
```
