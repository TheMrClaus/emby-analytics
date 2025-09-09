-- Create audit logging table for cleanup operations
CREATE TABLE IF NOT EXISTS cleanup_jobs (
    id TEXT PRIMARY KEY,                    -- UUID for job identification
    operation_type TEXT NOT NULL,          -- 'missing-items', 'merge-duplicates', etc.
    status TEXT NOT NULL,                   -- 'running', 'completed', 'failed'
    started_at INTEGER NOT NULL,           -- unix timestamp
    completed_at INTEGER,                  -- unix timestamp when finished
    total_items_checked INTEGER DEFAULT 0, -- how many items were examined
    items_processed INTEGER DEFAULT 0,     -- how many items were affected
    summary TEXT,                          -- JSON summary of operation
    created_by TEXT                        -- optional: who initiated the job
);

-- Individual audit entries for each item action
CREATE TABLE IF NOT EXISTS cleanup_audit_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,                  -- references cleanup_jobs.id
    action_type TEXT NOT NULL,             -- 'deleted', 'merged', 'skipped'
    item_id TEXT NOT NULL,                 -- the library item affected
    item_name TEXT,                        -- item name for reference
    item_type TEXT,                        -- Movie, Episode, etc.
    target_item_id TEXT,                   -- for merges: destination item
    metadata TEXT,                         -- JSON with additional details
    timestamp INTEGER NOT NULL,           -- unix timestamp of this action
    FOREIGN KEY (job_id) REFERENCES cleanup_jobs(id) ON DELETE CASCADE
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_cleanup_jobs_started ON cleanup_jobs(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_cleanup_jobs_type ON cleanup_jobs(operation_type);
CREATE INDEX IF NOT EXISTS idx_cleanup_audit_job ON cleanup_audit_items(job_id);
CREATE INDEX IF NOT EXISTS idx_cleanup_audit_item ON cleanup_audit_items(item_id);
CREATE INDEX IF NOT EXISTS idx_cleanup_audit_timestamp ON cleanup_audit_items(timestamp DESC);