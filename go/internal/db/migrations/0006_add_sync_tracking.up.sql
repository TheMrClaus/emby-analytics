-- Create sync_tracking table to store last sync timestamps
CREATE TABLE IF NOT EXISTS sync_tracking (
    id INTEGER PRIMARY KEY,
    sync_type TEXT NOT NULL UNIQUE, -- 'library_full', 'library_incremental', etc.
    last_sync_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    items_processed INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial record for library sync tracking
INSERT OR IGNORE INTO sync_tracking (sync_type, last_sync_at, items_processed) 
VALUES ('library_incremental', '1970-01-01 00:00:00', 0);

-- Create index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_sync_tracking_type ON sync_tracking(sync_type);