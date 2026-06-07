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
    containerd_ns,
    status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: GetContainerById :one
SELECT * FROM containers WHERE id = $1;

-- name: ListContainers :many
SELECT * FROM containers ORDER BY created_at DESC;

-- name: ListContainersByNodeId :many
SELECT * FROM containers WHERE node_id = $1 ORDER BY created_at DESC;

-- name: ListWorkloadsByNodeId :many
SELECT * FROM containers
WHERE node_id = $1
  AND status IN (
    'pending', 'scheduled', 'pulling', 'creating', 'running',
    'stopped', 'crashed', 'backoff', 'failed', 'removed'
  )
ORDER BY created_at ASC;

-- name: ScheduleContainer :one
UPDATE containers
SET status = 'scheduled', scheduled_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkContainerRemoved :one
UPDATE containers
SET status = 'removed', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountRunningContainersByNodeId :one
SELECT COUNT(*)::int FROM containers
WHERE node_id = $1
  AND status IN ('pending', 'scheduled', 'pulling', 'creating', 'running', 'backoff');

-- name: SumContainerResourcesByNodeId :one
SELECT
    COALESCE(SUM(cpu_limit), 0)::numeric AS total_cpu,
    COALESCE(SUM(memory_limit_mib), 0)::int AS total_memory_mib
FROM containers
WHERE node_id = $1
  AND status IN ('pending', 'scheduled', 'pulling', 'creating', 'running', 'backoff');

-- name: HasContainerWithImageOnNode :one
SELECT EXISTS(
    SELECT 1 FROM containers
    WHERE node_id = $1
      AND image = $2
      AND status IN ('pending', 'scheduled', 'pulling', 'creating', 'running', 'backoff')
) AS exists;

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
    last_failed_at = COALESCE($11, last_failed_at),
    scheduled_at = COALESCE($12, scheduled_at),
    started_at = COALESCE($13, started_at),
    stopped_at = COALESCE($14, stopped_at),
    updated_at = now()
WHERE id = $1
RETURNING *;
