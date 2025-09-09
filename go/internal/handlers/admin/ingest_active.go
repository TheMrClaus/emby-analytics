package admin

import (
    "database/sql"
    "time"

    "github.com/gofiber/fiber/v3"

    emby "emby-analytics/internal/emby"
)

type IngestResult struct {
    Inserted int `json:"inserted"`
    Updated  int `json:"updated"`
    Total    int `json:"total"`
}

// IngestActiveSessions upserts play_sessions rows for all currently active Emby sessions.
// This is useful to backfill a missing row when the periodic processor missed an item switch.
func IngestActiveSessions(db *sql.DB, em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
        sessions, err := em.GetActiveSessions()
        if err != nil {
            return c.Status(502).JSON(fiber.Map{"error": err.Error()})
        }
        now := time.Now().UTC().Unix()
        res := IngestResult{Total: len(sessions)}

        for _, s := range sessions {
            var existingID int64
            selErr := db.QueryRow(`SELECT id FROM play_sessions WHERE session_id=? AND item_id=?`, s.SessionID, s.ItemID).Scan(&existingID)
            if selErr == nil {
                // Update existing row
                _, _ = db.Exec(`
                    UPDATE play_sessions 
                    SET user_id=?, device_id=?, client_name=?, item_name=?, item_type=?, play_method=?,
                        ended_at=NULL, is_active=true, transcode_reasons=?, remote_address=?,
                        video_method=?, audio_method=?, video_codec_from=?, video_codec_to=?,
                        audio_codec_from=?, audio_codec_to=?
                    WHERE id=?
                `, s.UserID, s.Device, s.App, s.ItemName, s.ItemType, s.PlayMethod,
                   joinReasons(s.TransReasons), s.RemoteAddress,
                   s.VideoMethod, s.AudioMethod, s.TransVideoFrom, s.TransVideoTo, s.TransAudioFrom, s.TransAudioTo, existingID)
                res.Updated++
                continue
            }
            // Insert new row
            _, _ = db.Exec(`
                INSERT INTO play_sessions
                (user_id, session_id, device_id, client_name, item_id, item_name, item_type, play_method, started_at, is_active, transcode_reasons, remote_address, video_method, audio_method, video_codec_from, video_codec_to, audio_codec_from, audio_codec_to)
                VALUES(?,?,?,?,?,?,?,?,?,true,?,?,?,?,?,?,?)
            `, s.UserID, s.SessionID, s.Device, s.App, s.ItemID, s.ItemName, s.ItemType, s.PlayMethod, now,
               joinReasons(s.TransReasons), s.RemoteAddress, s.VideoMethod, s.AudioMethod, s.TransVideoFrom, s.TransVideoTo, s.TransAudioFrom, s.TransAudioTo)
            res.Inserted++
        }

        return c.JSON(res)
    }
}

func joinReasons(rs []string) string {
    if len(rs) == 0 { return "" }
    out := rs[0]
    for i := 1; i < len(rs); i++ { out += "," + rs[i] }
    return out
}
