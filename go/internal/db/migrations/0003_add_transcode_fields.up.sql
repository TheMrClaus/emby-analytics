-- Add transcode reasons and remote address fields to play_sessions table
ALTER TABLE play_sessions ADD COLUMN transcode_reasons TEXT;
ALTER TABLE play_sessions ADD COLUMN remote_address TEXT;

-- Add indexes for better performance on these new fields
CREATE INDEX IF NOT EXISTS idx_play_sessions_transcode_reasons ON play_sessions(transcode_reasons);
CREATE INDEX IF NOT EXISTS idx_play_sessions_remote_address ON play_sessions(remote_address);
