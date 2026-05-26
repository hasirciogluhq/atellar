-- name: UpdateNodeHeartbeat :exec
UPDATE nodes SET last_heartbeat = now() WHERE id = $1;

-- name: GetNodeById :one
SELECT * FROM nodes WHERE id = $1;

-- name: GetNodeByName :one
SELECT * FROM nodes WHERE name = $1;