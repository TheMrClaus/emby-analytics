-- Add columns to store media size and bitrate from Emby metadata
ALTER TABLE library_item ADD COLUMN file_size_bytes BIGINT;
ALTER TABLE library_item ADD COLUMN bitrate_bps INTEGER;

