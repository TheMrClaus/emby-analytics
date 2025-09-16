-- Add server_id to play_intervals and backfill from play_sessions
ALTER TABLE play_intervals ADD COLUMN server_id TEXT;

-- Backfill existing rows using session_fk
UPDATE play_intervals
SET server_id = (
    SELECT ps.server_id FROM play_sessions ps WHERE ps.id = play_intervals.session_fk
)
WHERE server_id IS NULL OR server_id = '';

-- Index for server-scoped queries
CREATE INDEX IF NOT EXISTS idx_play_intervals_server ON play_intervals(server_id);

