-- Indexes to speed up server resolution and time-ordered lookups

-- Fast path for resolving latest interval per item
CREATE INDEX IF NOT EXISTS idx_play_intervals_item_end ON play_intervals(item_id, end_ts DESC);

-- Fallback path when using play_sessions to resolve server by item
CREATE INDEX IF NOT EXISTS idx_play_sessions_item_server_time ON play_sessions(item_id, server_id, started_at DESC);

