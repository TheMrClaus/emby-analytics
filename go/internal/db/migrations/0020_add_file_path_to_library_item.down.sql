-- Rollback: Remove file_path column and index
DROP INDEX IF EXISTS idx_library_item_file_path;

-- Note: SQLite doesn't support DROP COLUMN directly in older versions
-- If using SQLite < 3.35.0, this would require table recreation
-- For now, we'll leave the column (it will just be NULL if rolled back)
-- ALTER TABLE library_item DROP COLUMN file_path;
