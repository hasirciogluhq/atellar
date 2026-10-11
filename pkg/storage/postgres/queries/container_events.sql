-- name: CreateContainerEvent :exec
INSERT INTO container_events (id, container_id, node_id, event, message, created_at)
VALUES ($1, $2, $3, $4, $5, clock_timestamp());

-- name: ListContainerEventsByContainerId :many
SELECT * FROM container_events
WHERE container_id = $1
ORDER BY created_at DESC, id DESC;
