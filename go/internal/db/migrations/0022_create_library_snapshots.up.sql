-- Create library_snapshots table for tracking storage and library metrics over time
CREATE TABLE library_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  snapshot_date DATE NOT NULL UNIQUE,
  total_items INTEGER NOT NULL DEFAULT 0,
  total_size_bytes INTEGER NOT NULL DEFAULT 0,
  movie_count INTEGER NOT NULL DEFAULT 0,
  series_count INTEGER NOT NULL DEFAULT 0,
  episode_count INTEGER NOT NULL DEFAULT 0,
  video_4k_count INTEGER NOT NULL DEFAULT 0,
  video_1080p_count INTEGER NOT NULL DEFAULT 0,
  video_720p_count INTEGER NOT NULL DEFAULT 0,
  video_sd_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_library_snapshots_date ON library_snapshots(snapshot_date);
