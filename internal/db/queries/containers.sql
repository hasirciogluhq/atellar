-- name: CreateContainer :one
INSERT INTO containers (
    id,
    node_id,
    image,
    command,
    entrypoint,
    env,
    working_dir,
    cpu_limit,
    cpu_shares,
    memory_limit_mib,
    restart_policy,
    containerd_ns
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetContainerById :one
SELECT * FROM containers WHERE id = $1;

-- name: ListContainers :many
SELECT * FROM containers ORDER BY created_at DESC;

-- name: ListContainersByNodeId :many
SELECT * FROM containers WHERE node_id = $1 ORDER BY created_at DESC;

-- name: UpdateContainerStatus :one
UPDATE containers
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateContainerRuntime :one
UPDATE containers
SET
    containerd_id = $2,
    snapshot_key = $3,
    task_pid = $4,
    image_digest = $5,
    overlay_ip = $6,
    status = $7,
    exit_code = $8,
    error_message = $9,
    restart_count = $10,
    scheduled_at = COALESCE($11, scheduled_at),
    started_at = COALESCE($12, started_at),
    stopped_at = COALESCE($13, stopped_at),
    updated_at = now()
WHERE id = $1
RETURNING *;
