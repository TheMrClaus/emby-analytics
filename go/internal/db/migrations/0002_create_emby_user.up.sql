-- Creates the emby_user table if it doesn't exist
CREATE TABLE IF NOT EXISTS emby_user (
  id TEXT PRIMARY KEY,           -- Emby user Id (GUID-like string)
  name TEXT NOT NULL,            -- Display name
  is_admin INTEGER NOT NULL DEFAULT 0, -- Optional: 0/1 flag if you have this info
  created_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
  updated_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
);

-- Keeps updated_at fresh on UPDATE
CREATE TRIGGER IF NOT EXISTS emby_user_set_updated_at
AFTER UPDATE ON emby_user
FOR EACH ROW
BEGIN
  UPDATE emby_user SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS library_item (
  id TEXT PRIMARY KEY,  -- Emby ItemId
  title TEXT NOT NULL,
  type TEXT,
  container TEXT,
  width INTEGER,
  height INTEGER,
  hdr10 INTEGER DEFAULT 0,
  dolby_vision INTEGER DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
  updated_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
);

CREATE TRIGGER IF NOT EXISTS library_item_set_updated_at
AFTER UPDATE ON library_item
FOR EACH ROW
BEGIN
  UPDATE library_item
  SET updated_at = (CURRENT_TIMESTAMP)
  WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS lifetime_watch (
  user_id TEXT NOT NULL,
  ms_watched INTEGER NOT NULL DEFAULT 0,
  sessions INTEGER NOT NULL DEFAULT 0,
  last_seen DATETIME,
  PRIMARY KEY (user_id),
  FOREIGN KEY (user_id) REFERENCES emby_user(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_library_item_type ON library_item(type);
CREATE INDEX IF NOT EXISTS idx_library_item_title ON library_item(title);