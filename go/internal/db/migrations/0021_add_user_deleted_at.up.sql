-- Add deleted_at column to emby_user for tracking deleted users
ALTER TABLE emby_user ADD COLUMN deleted_at TIMESTAMP;

-- Create index for filtering active users
CREATE INDEX IF NOT EXISTS idx_emby_user_deleted ON emby_user(deleted_at);
