-- Add user_name snapshot to play_sessions for accurate historical display across servers
ALTER TABLE play_sessions ADD COLUMN user_name TEXT;

-- Backfill from emby_user where possible (Emby rows)
UPDATE play_sessions
SET user_name = (
    SELECT eu.name FROM emby_user eu WHERE eu.id = play_sessions.user_id
)
WHERE (user_name IS NULL OR user_name = '')
  AND user_id IS NOT NULL AND user_id <> '';

