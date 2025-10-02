-- Add file_path column to library_item table for reliable deduplication across servers
ALTER TABLE library_item ADD COLUMN file_path TEXT;

-- Create index on file_path for efficient deduplication queries
CREATE INDEX IF NOT EXISTS idx_library_item_file_path ON library_item(file_path);
