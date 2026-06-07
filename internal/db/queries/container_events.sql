-- name: CreateContainerEvent :one
INSERT INTO container_events (id, container_id, node_id, event, message, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListContainerEventsByContainerId :many
SELECT * FROM container_events
WHERE container_id = $1
ORDER BY created_at DESC;
