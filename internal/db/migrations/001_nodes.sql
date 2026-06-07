CREATE TYPE node_status AS ENUM ('ready', 'not_ready', 'down', 'maintenance', 'evicting', 'evicted', 'pending');

CREATE TABLE nodes (
    id text PRIMARY KEY NOT NULL,
    name text NOT NULL,
    public_ip text DEFAULT NULL,
    private_ip text DEFAULT NULL,
    status node_status NOT NULL DEFAULT 'pending',
    last_heartbeat timestamp with time zone DEFAULT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE TABLE node_join_tokens (
    id text PRIMARY KEY NOT NULL,
    token text NOT NULL UNIQUE,
    expires_at timestamp with time zone DEFAULT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);
