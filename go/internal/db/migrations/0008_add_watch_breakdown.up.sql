-- Add columns to store Emby vs Trakt watch time breakdown
ALTER TABLE lifetime_watch ADD COLUMN emby_ms INTEGER DEFAULT 0;
ALTER TABLE lifetime_watch ADD COLUMN trakt_ms INTEGER DEFAULT 0;

-- Update existing records to preserve total in emby_ms for now
UPDATE lifetime_watch SET emby_ms = total_ms WHERE emby_ms IS NULL OR emby_ms = 0;