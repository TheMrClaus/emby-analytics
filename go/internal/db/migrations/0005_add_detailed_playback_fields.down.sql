-- Remove detailed playback fields (SQLite doesn't support DROP COLUMN directly, so we recreate the table)

-- Create new table without the detailed playback fields
CREATE TABLE play_sessions_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  session_id TEXT NOT NULL,
  device_id TEXT,
  client_name TEXT,
  item_id TEXT NOT NULL,
  item_name TEXT,
  item_type TEXT,
  play_method TEXT,
  started_at INTEGER NOT NULL,
  ended_at INTEGER,
  is_active BOOLEAN DEFAULT true,
  transcode_reasons TEXT,
  remote_address TEXT,
  UNIQUE(session_id, item_id)
);

-- Copy data from old table to new table (excluding the new fields)
INSERT INTO play_sessions_new SELECT 
  id, user_id, session_id, device_id, client_name, item_id, item_name, 
  item_type, play_method, started_at, ended_at, is_active, 
  transcode_reasons, remote_address
FROM play_sessions;

-- Drop old table and rename new one
DROP TABLE play_sessions;
ALTER TABLE play_sessions_new RENAME TO play_sessions;

-- Recreate original indexes
CREATE INDEX IF NOT EXISTS idx_play_sessions_user ON play_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_play_sessions_item ON play_sessions(item_id);
CREATE INDEX IF NOT EXISTS idx_play_sessions_transcode_reasons ON play_sessions(transcode_reasons);
CREATE INDEX IF NOT EXISTS idx_play_sessions_remote_address ON play_sessions(remote_address);
