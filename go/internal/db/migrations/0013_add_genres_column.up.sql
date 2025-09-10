-- Add optional genres column to library_item for future use
-- Stored as comma-separated values (e.g., "Action, Comedy")
ALTER TABLE library_item ADD COLUMN genres TEXT;

