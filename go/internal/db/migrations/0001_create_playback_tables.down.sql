DROP TABLE IF EXISTS play_intervals;
DROP TABLE IF EXISTS play_events;
DROP TABLE IF EXISTS play_sessions;

-- Re-create the old table if rolling back
CREATE TABLE IF NOT EXISTS play_event (
    ts INTEGER,
    user_id TEXT,
    item_id TEXT,
    pos_ms INTEGER
);
CREATE INDEX IF NOT EXISTS idx_play_event_user ON play_event(user_id);
CREATE INDEX IF NOT EXISTS idx_play_event_item ON play_event(item_id);
CREATE INDEX IF NOT EXISTS idx_play_event_ts ON play_event(ts);