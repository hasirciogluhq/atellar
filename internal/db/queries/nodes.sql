-- name: UpdateNodeHeartbeat :exec
UPDATE nodes SET last_heartbeat = now(), updated_at = now() WHERE id = $1;

-- name: GetNodeById :one
SELECT * FROM nodes WHERE id = $1;

-- name: GetNodeByName :one
SELECT * FROM nodes WHERE name = $1;

-- name: CreateNode :one
INSERT INTO nodes (id, name, public_ip, private_ip, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: CreateJoinToken :one
INSERT INTO node_join_tokens (id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetJoinTokenByToken :one
SELECT * FROM node_join_tokens WHERE token = $1;

-- name: ListJoinTokens :many
SELECT * FROM node_join_tokens ORDER BY created_at DESC;
