package db

import (
	"context"
	"database/sql"
	"time"
)

type LibraryItem struct {
	ID           string
	ServerID     string
	ItemID       string
	Name         sql.NullString
	MediaType    sql.NullString
	Height       sql.NullInt64  // legacy
	Width        sql.NullInt64  // NEW
	DisplayTitle sql.NullString // NEW (optional)
	RunTimeTicks sql.NullInt64
	Container    sql.NullString
	VideoCodec   sql.NullString
	AudioCodec   sql.NullString
}

func UpsertLibraryItem(ctx context.Context, sqldb *sql.DB, it LibraryItem) error {
	_, err := sqldb.ExecContext(ctx, `
INSERT INTO library_item (
  id, server_id, item_id, name, media_type, height, width, display_title,
  run_time_ticks, container, video_codec, audio_codec, created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
) ON CONFLICT(id) DO UPDATE SET
  server_id = excluded.server_id,
  item_id = excluded.item_id,
  name = excluded.name,
  media_type = excluded.media_type,
  height = excluded.height,
  width = excluded.width,               -- NEW
  display_title = excluded.display_title,
  run_time_ticks = excluded.run_time_ticks,
  container = excluded.container,
  video_codec = excluded.video_codec,
  audio_codec = excluded.audio_codec,
  updated_at = excluded.updated_at
`, it.ID, it.ServerID, it.ItemID, it.Name, it.MediaType, it.Height, it.Width, it.DisplayTitle,
		it.RunTimeTicks, it.Container, it.VideoCodec, it.AudioCodec,
		time.Now(), time.Now(),
	)
	return err
}
