-- ENUMS
CREATE TYPE node_status AS ENUM (
    'pending',
    'ready',
    'not_ready',
    'maintenance',
    'evicting',
    'evicted',
    'down'
);

CREATE TYPE container_status AS ENUM (
    'pending',      -- DB'ye yazıldı, node henüz görmedi
    'scheduled',    -- node'a atandı, henüz başlamadı
    'pulling',      -- image pull ediliyor
    'creating',     -- containerd container create edildi, task başlamadı
    'running',      -- task başladı, pid var
    'stopped',      -- graceful stop
    'crashed',      -- exit_code != 0
    'terminated'    -- silindi, temizlendi
);

-- NODES
CREATE TABLE nodes (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL UNIQUE,
    public_ip INET DEFAULT NULL,
    private_ip INET DEFAULT NULL,
    overlay_ip INET DEFAULT NULL,
    overlay_subnet CIDR DEFAULT NULL,
    status node_status NOT NULL DEFAULT 'pending',
    last_heartbeat TIMESTAMPTZ DEFAULT NULL,
    -- node agent versiyonu, debug için
    agent_version TEXT DEFAULT NULL,
    -- containerd socket path (default /run/containerd/containerd.sock)
    containerd_sock TEXT NOT NULL DEFAULT '/run/containerd/containerd.sock',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- NODE JOIN TOKENS
CREATE TABLE node_join_tokens (
    id TEXT PRIMARY KEY NOT NULL,
    token TEXT NOT NULL UNIQUE,
    -- tek kullanımlık mı?
    single_use BOOLEAN NOT NULL DEFAULT true,
    used_at TIMESTAMPTZ DEFAULT NULL,
    used_by TEXT REFERENCES nodes (id) DEFAULT NULL,
    expires_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- OVERLAY IP POOL (node subnet'inden pre-allocate)
CREATE TABLE overlay_ip_pool (
    ip INET PRIMARY KEY NOT NULL,
    node_id TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    container_id TEXT DEFAULT NULL, -- NULL = boş
    allocated_at TIMESTAMPTZ DEFAULT NULL
);

-- CONTAINERS
CREATE TABLE containers (
    id              TEXT PRIMARY KEY NOT NULL,
    node_id         TEXT NOT NULL REFERENCES nodes(id),

-- containerd specifics
-- namespace: containerd multi-tenant namespace (default "atellar")
containerd_ns TEXT NOT NULL DEFAULT 'atellar',
-- containerd container ID (bizim id ile aynı olabilir ama explicit tutalım)
containerd_id TEXT DEFAULT NULL,
-- snapshot key (overlayfs snapshot, containerd snapshotters)
snapshot_key TEXT DEFAULT NULL,
-- task PID (containerd task = OCI runtime process)
task_pid INTEGER DEFAULT NULL,

-- Image & runtime
image TEXT NOT NULL,
-- image digest (pull sonrası doldurulur, sha256:...)
image_digest TEXT DEFAULT NULL,
command TEXT [] DEFAULT NULL,
entrypoint TEXT [] DEFAULT NULL,
env JSONB NOT NULL DEFAULT '{}',
working_dir TEXT DEFAULT NULL,

-- Network
overlay_ip INET DEFAULT NULL,
exposed_ports JSONB DEFAULT NULL,
-- [{"proto":"tcp","port":8080,"host_port":30080}]

-- Resources
cpu_limit NUMERIC DEFAULT NULL, -- cores (0.5, 1, 2 ...)
cpu_shares INTEGER DEFAULT 1024, -- cgroup cpu.shares
memory_limit_mib INTEGER DEFAULT NULL, -- MiB

-- Lifecycle
status container_status NOT NULL DEFAULT 'pending',
exit_code INTEGER DEFAULT NULL,
-- crash durumunda containerd'dan gelen hata mesajı
error_message TEXT DEFAULT NULL,
-- kaç kez restart edildi
restart_count INTEGER NOT NULL DEFAULT 0,
restart_policy TEXT NOT NULL DEFAULT 'no',
-- no | always | on-failure

-- Timestamps
created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    scheduled_at    TIMESTAMPTZ DEFAULT NULL,
    started_at      TIMESTAMPTZ DEFAULT NULL,
    stopped_at      TIMESTAMPTZ DEFAULT NULL
);

-- CONTAINER EVENTS (audit + debug, opsiyonel ama çok işe yarar)
CREATE TYPE container_event_type AS ENUM (
    'scheduled',
    'pull_started',
    'pull_finished',
    'pull_failed',
    'created',
    'started',
    'stopped',
    'crashed',
    'terminated',
    'restart'
);

CREATE TABLE container_events (
    id TEXT PRIMARY KEY NOT NULL,
    container_id TEXT NOT NULL REFERENCES containers (id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes (id),
    event container_event_type NOT NULL,
    message TEXT DEFAULT NULL,
    metadata JSONB DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- INDEXES
CREATE INDEX idx_containers_node_id ON containers (node_id);

CREATE INDEX idx_containers_status ON containers (status);

CREATE INDEX idx_containers_overlay_ip ON containers (overlay_ip);

CREATE INDEX idx_container_events_cid ON container_events (container_id);

CREATE INDEX idx_container_events_time ON container_events (created_at DESC);

CREATE INDEX idx_overlay_pool_node ON overlay_ip_pool (node_id);

CREATE INDEX idx_overlay_pool_free ON overlay_ip_pool (node_id)
WHERE
    container_id IS NULL;

CREATE INDEX idx_nodes_status ON nodes (status);