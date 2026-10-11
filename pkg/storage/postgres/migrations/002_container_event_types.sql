-- Lifecycle steps the API server records that 001 had no event type for.
ALTER TYPE container_event_type ADD VALUE IF NOT EXISTS 'backoff';
ALTER TYPE container_event_type ADD VALUE IF NOT EXISTS 'failed';
ALTER TYPE container_event_type ADD VALUE IF NOT EXISTS 'removed';
