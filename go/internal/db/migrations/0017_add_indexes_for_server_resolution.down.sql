-- Down migration: drop created indexes
DROP INDEX IF EXISTS idx_play_intervals_item_end;
DROP INDEX IF EXISTS idx_play_sessions_item_server_time;

