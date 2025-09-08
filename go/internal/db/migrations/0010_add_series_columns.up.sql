-- Add series linkage columns to library_item for Episodes
ALTER TABLE library_item ADD COLUMN series_id TEXT;
ALTER TABLE library_item ADD COLUMN series_name TEXT;
CREATE INDEX IF NOT EXISTS idx_library_item_series_id ON library_item(series_id);

