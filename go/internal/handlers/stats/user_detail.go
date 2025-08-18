package stats

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type UserTopItem struct {
	ItemID string  `json:"item_id"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Hours  float64 `json:"hours"`
}

type UserActivity struct {
	Timestamp int64   `json:"timestamp"`
	ItemID    string  `json:"item_id"`
	ItemName  string  `json:"item_name"`
	ItemType  string  `json:"item_type"`
	PosHours  float64 `json:"pos_hours"`
}

type UserDetail struct {
	UserID         string         `json:"user_id"`
	UserName       string         `json:"user_name"`
	TotalHours     float64        `json:"total_hours"`
	Plays          int            `json:"plays"`
	TopItems       []UserTopItem  `json:"top_items"`
	RecentActivity []UserActivity `json:"recent_activity"`
}

// GET /stats/users/:id?days=30&limit=10
func UserDetailHandler(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := c.Params("id", "")
		if userID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing user id"})
		}
		days := parseQueryInt(c, "days", 30)
		limit := parseQueryInt(c, "limit", 10)
		if days <= 0 {
			days = 30
		}
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		fromMs := time.Now().AddDate(0, 0, -days).UnixMilli()

		// Base info: user name, total hours and plays in window
		detail := UserDetail{
			UserID:         userID,
			UserName:       "",
			TotalHours:     0,
			Plays:          0,
			TopItems:       []UserTopItem{},
			RecentActivity: []UserActivity{},
		}

		// user name
		_ = db.QueryRow(`SELECT name FROM emby_user WHERE id = ?`, userID).Scan(&detail.UserName)

		// totals in window - use session count instead of position sum
		_ = db.QueryRow(`
			SELECT COUNT(*) * 0.75 AS hours,
			       COUNT(*) AS plays
			FROM play_event
			WHERE user_id = ? AND ts >= ?
		`, userID, fromMs).Scan(&detail.TotalHours, &detail.Plays)

		// top items - use event count instead of position sum
		if rows, err := db.Query(`
			SELECT li.id, li.name, li.type,
			       COUNT(*) * 0.5 AS hours
			FROM play_event pe
			JOIN library_item li ON li.id = pe.item_id
			WHERE pe.user_id = ? AND pe.ts >= ?
			GROUP BY li.id, li.name, li.type
			ORDER BY hours DESC
			LIMIT ?;
		`, userID, fromMs, limit); err == nil {
			defer rows.Close()
			for rows.Next() {
				var ti UserTopItem
				if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &ti.Hours); err == nil {
					detail.TopItems = append(detail.TopItems, ti)
				}
			}
		}

		// recent activity
		if rows, err := db.Query(`
			SELECT pe.ts, li.id, li.name, li.type, pe.pos_ms/3600000.0
			FROM play_event pe
			LEFT JOIN library_item li ON li.id = pe.item_id
			WHERE pe.user_id = ? AND pe.ts >= ?
			ORDER BY pe.ts DESC
			LIMIT ?;
		`, userID, fromMs, limit); err == nil {
			defer rows.Close()
			for rows.Next() {
				var a UserActivity
				if err := rows.Scan(&a.Timestamp, &a.ItemID, &a.ItemName, &a.ItemType, &a.PosHours); err == nil {
					detail.RecentActivity = append(detail.RecentActivity, a)
				}
			}
		}

		return c.JSON(detail)
	}
}
