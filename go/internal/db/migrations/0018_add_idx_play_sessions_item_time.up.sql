-- Optimize lookup of latest session name/type per item
CREATE INDEX IF NOT EXISTS idx_play_sessions_item_time ON play_sessions(item_id, started_at DESC);

