package admin

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type DebugSessionRow struct {
	ID             int64  `json:"id"`
	SessionID      string `json:"session_id"`
	ItemID         string `json:"item_id"`
	ItemName       string `json:"item_name"`
	ItemType       string `json:"item_type"`
	UserID         string `json:"user_id"`
	ClientName     string `json:"client_name"`
	DeviceID       string `json:"device_id"`
	StartedAt      int64  `json:"started_at"`
	EndedAt        *int64 `json:"ended_at,omitempty"`
	IsActive       bool   `json:"is_active"`
	PlayMethod     string `json:"play_method"`
	VideoMethod    string `json:"video_method"`
	AudioMethod    string `json:"audio_method"`
	VideoCodecFrom string `json:"video_codec_from"`
	VideoCodecTo   string `json:"video_codec_to"`
	AudioCodecFrom string `json:"audio_codec_from"`
	AudioCodecTo   string `json:"audio_codec_to"`
	Reasons        string `json:"transcode_reasons"`
}

// DebugSessions lists recent play_sessions with flexible filters.
// Query params:
//
//	q:          substring to search in item_name (optional, case-insensitive)
//	item_id:    filter by exact item_id (optional)
//	session_id: filter by exact session_id (optional)
//	days:       window in days (default 1)
//	limit:      max rows (default 100)
//	activeOnly: if "1"/"true", only return is_active rows
func DebugSessions(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		q := strings.TrimSpace(c.Query("q"))
		itemID := strings.TrimSpace(c.Query("item_id"))
		sessionID := strings.TrimSpace(c.Query("session_id"))
		days := 1
		if d := c.Query("days"); d != "" {
			if n, err := strconv.Atoi(d); err == nil && n > 0 {
				days = n
			}
		}
		limit := 100
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}

		// Basic query with timeframe
		base := `
            SELECT id, session_id, item_id, item_name, item_type, user_id, client_name, device_id,
                   started_at, ended_at, is_active, play_method,
                   COALESCE(video_method,''), COALESCE(audio_method,''),
                   COALESCE(video_codec_from,''), COALESCE(video_codec_to,''),
                   COALESCE(audio_codec_from,''), COALESCE(audio_codec_to,''),
                   COALESCE(transcode_reasons,'')
            FROM play_sessions
            WHERE started_at >= (strftime('%s','now') - (? * 86400))
        `

		var rows *sql.Rows
		var err error
		// Apply optional filters
		args := []any{days}
		if q != "" {
			base += " AND lower(COALESCE(item_name,'')) LIKE ?"
			args = append(args, "%"+strings.ToLower(q)+"%")
		}
		if itemID != "" {
			base += " AND item_id = ?"
			args = append(args, itemID)
		}
		if sessionID != "" {
			base += " AND session_id = ?"
			args = append(args, sessionID)
		}
		if act := strings.ToLower(strings.TrimSpace(c.Query("activeOnly"))); act == "1" || act == "true" || act == "yes" {
			base += " AND is_active = 1"
		}
		base += " ORDER BY started_at DESC LIMIT ?"
		args = append(args, limit)
		rows, err = db.Query(base, args...)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		out := make([]DebugSessionRow, 0)
		for rows.Next() {
			var r DebugSessionRow
			var ended sql.NullInt64
			var isActiveInt int
			if err := rows.Scan(&r.ID, &r.SessionID, &r.ItemID, &r.ItemName, &r.ItemType, &r.UserID,
				&r.ClientName, &r.DeviceID, &r.StartedAt, &ended, &isActiveInt, &r.PlayMethod,
				&r.VideoMethod, &r.AudioMethod, &r.VideoCodecFrom, &r.VideoCodecTo,
				&r.AudioCodecFrom, &r.AudioCodecTo, &r.Reasons); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			if ended.Valid {
				r.EndedAt = &ended.Int64
			}
			r.IsActive = isActiveInt != 0
			out = append(out, r)
		}
		if err := rows.Err(); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(out)
	}
}
