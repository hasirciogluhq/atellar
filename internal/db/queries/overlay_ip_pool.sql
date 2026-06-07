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
