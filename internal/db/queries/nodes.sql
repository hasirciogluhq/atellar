-- name: UpdateNodeHeartbeat :exec
UPDATE nodes SET last_heartbeat = now(), updated_at = now() WHERE id = $1;

-- name: GetNodeById :one
SELECT * FROM nodes WHERE id = $1;

-- name: GetNodeByName :one
SELECT * FROM nodes WHERE name = $1;

-- name: ListNodes :many
SELECT * FROM nodes ORDER BY created_at DESC;

-- name: CreateNode :one
INSERT INTO nodes (id, name, public_ip, private_ip, agent_version, containerd_sock, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
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
