-- Drop in reverse dependency order

-- 1. Drop indexes
DROP INDEX IF EXISTS idx_library_item_type;
DROP INDEX IF EXISTS idx_library_item_title;

-- 2. Drop lifetime_watch first (depends on emby_user)
DROP TABLE IF EXISTS lifetime_watch;

-- 3. Drop triggers
DROP TRIGGER IF EXISTS library_item_set_updated_at;
DROP TRIGGER IF EXISTS emby_user_set_updated_at;

-- 4. Drop main tables
DROP TABLE IF EXISTS library_item;
DROP TABLE IF EXISTS emby_user;
