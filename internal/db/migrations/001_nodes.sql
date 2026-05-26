CREATE type node_status as enum ('ready', 'not_ready', 'down', 'maintenance', 'evicting', 'evicted', 'pending');

CREATE table nodes (
    id text primary key not null,
    name text not null,
    public_ip text default null,
    private_ip text default null,
    status node_status not null default 'pending',
    last_heartbeat timestamp with time zone not null default now(),
    created_at timestamp with time zone not null default now(),
    updated_at timestamp with time zone not null default now()
);