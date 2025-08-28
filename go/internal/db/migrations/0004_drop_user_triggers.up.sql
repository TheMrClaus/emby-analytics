-- Drop any triggers that reference columns we don't have
-- This fixes the "no such column: updated_at" error
DROP TRIGGER IF EXISTS emby_user_set_updated_at;
DROP TRIGGER IF EXISTS library_item_set_updated_at;
