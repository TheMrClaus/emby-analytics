-- SQLite cannot drop a column without table rebuild; leave as no-op for down migration.
-- If needed, a full table recreate would be required to remove server_id from play_intervals.

