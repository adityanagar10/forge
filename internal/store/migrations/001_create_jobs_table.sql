CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'DEAD')),
    payload BYTEA,
    attempts SMALLINT NOT NULL DEFAULT 0,
    max_retries SMALLINT NOT NULL DEFAULT 3,
    priority SMALLINT NOT NULL DEFAULT 0,
    queue TEXT NOT NULL DEFAULT 'default',
    scheduled_at BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    last_error TEXT
);

-- Index for dequeuing jobs (pending jobs ordered by priority and schedule time)
CREATE INDEX IF NOT EXISTS idx_jobs_dequeue
    ON jobs(status, priority DESC, scheduled_at)
    WHERE status = 'PENDING';

-- Index for finding jobs by type
CREATE INDEX IF NOT EXISTS idx_jobs_type
    ON jobs(type);

-- Index for DLQ queries
CREATE INDEX IF NOT EXISTS idx_jobs_dead
    ON jobs(status)
    WHERE status = 'DEAD';

-- Index for scheduled job lookups
CREATE INDEX IF NOT EXISTS idx_jobs_scheduled
    ON jobs(scheduled_at)
    WHERE status = 'PENDING';
