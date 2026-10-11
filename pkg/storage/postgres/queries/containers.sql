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

-- name: ListWorkloads :many
SELECT * FROM containers
WHERE status <> 'terminated'
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

-- name: UpdateContainerRuntime :one
-- Identity fields (containerd id, snapshot, digest, overlay ip) and the
-- restart counter keep their value when a report leaves them out.
UPDATE containers
SET
    containerd_id = COALESCE(sqlc.narg(containerd_id), containerd_id),
    snapshot_key = COALESCE(sqlc.narg(snapshot_key), snapshot_key),
    task_pid = sqlc.narg(task_pid),
    image_digest = COALESCE(sqlc.narg(image_digest), image_digest),
    overlay_ip = COALESCE(sqlc.narg(overlay_ip), overlay_ip),
    status = sqlc.arg(status),
    exit_code = sqlc.narg(exit_code),
    error_message = sqlc.narg(error_message),
    restart_count = COALESCE(sqlc.narg(restart_count), restart_count),
    last_failed_at = COALESCE(sqlc.narg(last_failed_at), last_failed_at),
    scheduled_at = COALESCE(sqlc.narg(scheduled_at), scheduled_at),
    started_at = COALESCE(sqlc.narg(started_at), started_at),
    stopped_at = COALESCE(sqlc.narg(stopped_at), stopped_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
