-- name: UpdateNodeHeartbeat :exec
UPDATE nodes SET last_heartbeat = now(), updated_at = now() WHERE id = $1;

-- name: GetNodeById :one
SELECT * FROM nodes WHERE id = $1;

-- name: GetNodeByName :one
SELECT * FROM nodes WHERE name = $1;

-- name: GetNodeByTokenHash :one
SELECT * FROM nodes
WHERE token_hash = $1
  AND token_expires_at IS NOT NULL
  AND token_expires_at > now();

-- name: ListNodes :many
SELECT * FROM nodes ORDER BY created_at DESC;

-- name: CreateNode :one
INSERT INTO nodes (id, name, public_ip, private_ip, agent_version, containerd_sock, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateNodeToken :one
UPDATE nodes
SET token_hash = $2, token_expires_at = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateJoinToken :one
INSERT INTO node_join_tokens (id, token_hash, single_use, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetJoinTokenByHash :one
SELECT * FROM node_join_tokens WHERE token_hash = $1;

-- name: ListJoinTokens :many
SELECT * FROM node_join_tokens ORDER BY created_at DESC;

-- name: MarkJoinTokenUsed :exec
UPDATE node_join_tokens
SET used_at = now(), used_by = $2
WHERE id = $1;

-- name: ListActiveNodeOverlaySubnets :many
SELECT overlay_subnet::text FROM nodes
WHERE overlay_subnet IS NOT NULL
  AND status NOT IN ('evicted', 'evicting', 'down');

-- name: ListReclaimableOverlayNetworks :many
SELECT id, overlay_subnet::text FROM nodes
WHERE overlay_subnet IS NOT NULL
  AND status = 'evicted'
ORDER BY updated_at ASC;

-- name: EvictNode :one
UPDATE nodes
SET status = 'evicted', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ClearNodeOverlayNetwork :exec
UPDATE nodes
SET overlay_ip = NULL, overlay_subnet = NULL, updated_at = now()
WHERE id = $1;

-- name: ListNodeOverlaySubnets :many
SELECT overlay_subnet::text FROM nodes WHERE overlay_subnet IS NOT NULL;

-- name: UpdateNodeOverlayNetwork :one
UPDATE nodes
SET
    overlay_ip = $2,
    overlay_subnet = $3,
    status = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateNodeHardware :one
UPDATE nodes
SET
    cpu_cores = $2,
    memory_total_mib = $3,
    disk_total_gib = $4,
    hostname = $5,
    os = $6,
    arch = $7,
    kernel_version = $8,
    hardware_reported_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListSchedulableNodes :many
SELECT * FROM nodes
WHERE status = 'ready'
  AND hardware_reported_at IS NOT NULL
  AND cpu_cores IS NOT NULL
  AND memory_total_mib IS NOT NULL
ORDER BY created_at ASC;
