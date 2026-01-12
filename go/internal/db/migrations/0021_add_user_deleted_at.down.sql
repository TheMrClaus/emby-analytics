-- Remove deleted_at column from emby_user
-- SQLite doesn't support DROP COLUMN directly, so we recreate the table

DROP INDEX IF EXISTS idx_emby_user_deleted;

-- Create temporary table without deleted_at
CREATE TABLE emby_user_new (
    id TEXT PRIMARY KEY,
    server_id TEXT DEFAULT 'default-emby',
    server_type TEXT DEFAULT 'emby',
    name TEXT NOT NULL
);

-- Copy data
INSERT INTO emby_user_new (id, server_id, server_type, name)
SELECT id, server_id, server_type, name FROM emby_user;

-- Drop old table and rename
DROP TABLE emby_user;
ALTER TABLE emby_user_new RENAME TO emby_user;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_emby_user_server ON emby_user(server_id);
