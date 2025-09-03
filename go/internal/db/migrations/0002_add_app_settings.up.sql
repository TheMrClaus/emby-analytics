-- Add app_settings table for user-configurable settings
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index for faster lookups
CREATE INDEX idx_app_settings_key ON app_settings(key);

-- Insert default setting for Trakt inclusion (disabled by default)
INSERT INTO app_settings (key, value) VALUES ('include_trakt_items', 'false');