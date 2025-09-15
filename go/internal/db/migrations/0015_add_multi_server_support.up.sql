-- Multi-server support: Add server identification to tables that need it

-- Add server columns to emby_user table (rename to media_user for genericity)
ALTER TABLE emby_user ADD COLUMN server_id TEXT DEFAULT 'default-emby';
ALTER TABLE emby_user ADD COLUMN server_type TEXT DEFAULT 'emby';

-- Add server columns to play_sessions table
ALTER TABLE play_sessions ADD COLUMN server_id TEXT DEFAULT 'default-emby';
ALTER TABLE play_sessions ADD COLUMN server_type TEXT DEFAULT 'emby';

-- Create indexes for server-based queries
CREATE INDEX IF NOT EXISTS idx_emby_user_server ON emby_user(server_id);
CREATE INDEX IF NOT EXISTS idx_play_sessions_server ON play_sessions(server_id);
CREATE INDEX IF NOT EXISTS idx_play_sessions_server_type ON play_sessions(server_type);

-- Add server columns to library_item table (update existing server_id column to have proper default)
-- Note: library_item already has server_id column but may need server_type
ALTER TABLE library_item ADD COLUMN server_type TEXT DEFAULT 'emby';
CREATE INDEX IF NOT EXISTS idx_library_item_server_type ON library_item(server_type);

-- Update existing records to have proper server identification
UPDATE emby_user SET server_id = 'default-emby', server_type = 'emby' WHERE server_id IS NULL OR server_id = '';
UPDATE play_sessions SET server_id = 'default-emby', server_type = 'emby' WHERE server_id IS NULL OR server_id = '';
UPDATE library_item SET server_type = 'emby' WHERE server_type IS NULL OR server_type = '';

-- Make server_id and server_type NOT NULL after setting defaults
-- Note: SQLite doesn't support ALTER COLUMN, so we'll leave them as nullable but with defaults