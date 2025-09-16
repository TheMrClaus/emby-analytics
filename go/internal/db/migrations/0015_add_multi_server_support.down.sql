-- Rollback multi-server support changes

-- Drop indexes created for multi-server support
DROP INDEX IF EXISTS idx_emby_user_server;
DROP INDEX IF EXISTS idx_play_sessions_server;
DROP INDEX IF EXISTS idx_play_sessions_server_type;
DROP INDEX IF EXISTS idx_library_item_server_type;

-- Note: SQLite doesn't support dropping columns directly
-- In a real rollback scenario, you would need to:
-- 1. Create new tables without the server columns
-- 2. Copy data from old tables to new tables
-- 3. Drop old tables and rename new tables
-- For now, we'll just document this limitation

-- Remove server columns from emby_user (SQLite limitation - cannot drop columns)
-- ALTER TABLE emby_user DROP COLUMN server_id;
-- ALTER TABLE emby_user DROP COLUMN server_type;

-- Remove server columns from play_sessions (SQLite limitation)
-- ALTER TABLE play_sessions DROP COLUMN server_id; 
-- ALTER TABLE play_sessions DROP COLUMN server_type;

-- Remove server_type column from library_item (SQLite limitation)
-- ALTER TABLE library_item DROP COLUMN server_type;

-- SQLite rollback limitation notice
-- To fully rollback this migration in SQLite, you would need to:
-- 1. Export data excluding server columns
-- 2. Recreate tables with original schema
-- 3. Re-import data
-- This is complex and should be done manually if needed