-- name: DeleteOverlayIPPoolByNodeId :exec
DELETE FROM overlay_ip_pool WHERE node_id = $1;

-- name: CreateOverlayIPPoolEntry :one
INSERT INTO overlay_ip_pool (ip, node_id)
VALUES ($1, $2)
RETURNING *;

-- name: ListFreeOverlayIPsByNodeId :many
SELECT * FROM overlay_ip_pool
WHERE node_id = $1 AND container_id IS NULL
ORDER BY ip;

-- name: AllocateOverlayIP :one
UPDATE overlay_ip_pool
SET container_id = $2, allocated_at = now()
WHERE ip = $1 AND container_id IS NULL
RETURNING *;

-- name: ReleaseOverlayIP :exec
UPDATE overlay_ip_pool
SET container_id = NULL, allocated_at = NULL
WHERE ip = $1;

-- name: ListOverlayIPsByNodeId :many
SELECT * FROM overlay_ip_pool
WHERE node_id = $1
ORDER BY ip;

-- name: CountFreeOverlayIPsByNodeId :one
SELECT COUNT(*)::int FROM overlay_ip_pool
WHERE node_id = $1 AND container_id IS NULL;

-- name: AllocateFirstFreeOverlayIP :one
UPDATE overlay_ip_pool
SET container_id = sqlc.arg(container_id), allocated_at = now()
WHERE ip = (
    SELECT p.ip FROM overlay_ip_pool AS p
    WHERE p.node_id = sqlc.arg(node_id) AND p.container_id IS NULL
    ORDER BY p.ip
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
