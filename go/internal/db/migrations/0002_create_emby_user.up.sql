-- This migration is now a no-op since 0001 already creates the base tables
-- Migration 0001 creates: emby_user(id, name), library_item(id, name, type, height, codec), lifetime_watch(user_id, total_ms)
-- This file will be empty to avoid conflicts

-- If you need the extended schema in the future, create a new migration like 0005_add_user_timestamps.up.sql