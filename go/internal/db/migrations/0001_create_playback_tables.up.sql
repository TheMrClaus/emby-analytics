-- sessions: logical play grouping
CREATE TABLE IF NOT EXISTS play_sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  session_id TEXT NOT NULL,   -- Emby SessionId
  device_id TEXT,
  client_name TEXT,
  item_id TEXT NOT NULL,
  item_name TEXT,
  item_type TEXT,
  play_method TEXT,           -- DirectPlay/DirectStream/Transcode
  started_at INTEGER NOT NULL, -- unix seconds
  ended_at INTEGER,            -- unix seconds
  is_active BOOLEAN DEFAULT true,
  UNIQUE(session_id, item_id)  -- avoids accidental dup per Emby session
);

-- raw events we receive (for auditing / rebuild)
CREATE TABLE IF NOT EXISTS play_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_fk INTEGER NOT NULL REFERENCES play_sessions(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,          -- start|progress|stop
  is_paused INTEGER NOT NULL DEFAULT 0,
  position_ticks INTEGER,      -- Emby ticks
  playback_rate REAL,
  created_at INTEGER NOT NULL  -- unix seconds
);

-- derived intervals of actual watch time (pause excluded)
CREATE TABLE IF NOT EXISTS play_intervals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_fk INTEGER NOT NULL REFERENCES play_sessions(id) ON DELETE CASCADE,
  item_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  start_ts INTEGER NOT NULL,    -- unix seconds
  end_ts INTEGER NOT NULL,      -- unix seconds
  start_pos_ticks INTEGER,      -- position at start
  end_pos_ticks INTEGER,        -- position at end
  duration_seconds INTEGER NOT NULL, -- (end_ts - start_ts) adjusted
  seeked INTEGER NOT NULL DEFAULT 0
);

-- Drop the old table at the end of the migration
DROP TABLE IF EXISTS play_event;

-- Add new indexes for performance
CREATE INDEX IF NOT EXISTS idx_play_intervals_item_time ON play_intervals(item_id, start_ts, end_ts);
CREATE INDEX IF NOT EXISTS idx_play_intervals_user_time ON play_intervals(user_id, start_ts, end_ts);
CREATE INDEX IF NOT EXISTS idx_play_sessions_user ON play_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_play_intervals_time ON play_intervals(start_ts, end_ts);
CREATE INDEX IF NOT EXISTS idx_play_sessions_item ON play_sessions(item_id);