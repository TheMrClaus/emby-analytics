package stats

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"emby-analytics/internal/emby"

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
	UserID              string         `json:"user_id"`
	UserName            string         `json:"user_name"`
	TotalHours          float64        `json:"total_hours"`
	Plays               int            `json:"plays"`
	TotalMovies         int            `json:"total_movies"`
	TotalSeriesFinished int            `json:"total_series_finished"`
	TotalEpisodes       int            `json:"total_episodes"`
	TopItems            []UserTopItem  `json:"top_items"`
	RecentActivity      []UserActivity `json:"recent_activity"`
	LastSeenMovies      []UserTopItem  `json:"last_seen_movies"`
	LastSeenEpisodes    []UserTopItem  `json:"last_seen_episodes"`
	FinishedSeries      []UserTopItem  `json:"finished_series"`
}

// GET /stats/users/:id?days=30&limit=10
func UserDetailHandler(db *sql.DB, em *emby.Client) fiber.Handler {
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
			UserID:              userID,
			UserName:            "",
			TotalHours:          0,
			Plays:               0,
			TotalMovies:         0,
			TotalSeriesFinished: 0,
			TotalEpisodes:       0,
			TopItems:            []UserTopItem{},
			RecentActivity:      []UserActivity{},
			LastSeenMovies:      []UserTopItem{},
			LastSeenEpisodes:    []UserTopItem{},
			FinishedSeries:      []UserTopItem{},
		}

		// user name
		_ = db.QueryRow(`SELECT name FROM emby_user WHERE id = ?`, userID).Scan(&detail.UserName)

		// Use accurate lifetime watch data for user totals
		_ = db.QueryRow(`
            SELECT 
                COALESCE(lw.total_ms / 3600000.0, 0) AS hours,
                COALESCE(
                    (SELECT COUNT(DISTINCT ps.item_id)
                     FROM play_sessions ps 
                     LEFT JOIN library_item li ON li.id = ps.item_id
                     WHERE ps.user_id = ? AND ps.started_at >= ? AND ps.ended_at IS NOT NULL
                       AND (li.id IS NULL OR `+excludeLiveTvFilter()+`)
                    ), 
                    0
                ) AS plays
            FROM emby_user u
            LEFT JOIN lifetime_watch lw ON lw.user_id = u.id
            WHERE u.id = ?
        `, userID, fromMs/1000, userID).Scan(&detail.TotalHours, &detail.Plays)

		// Get user's top items based on play sessions
		if rows, err := db.Query(`
            SELECT 
                li.id, 
                li.name, 
                li.media_type,
                COUNT(*) as play_count
            FROM play_sessions ps
            JOIN library_item li ON li.id = ps.item_id
            WHERE ps.user_id = ? AND ps.started_at >= ? 
            AND ps.ended_at IS NOT NULL
            AND `+excludeLiveTvFilter()+`
            GROUP BY li.id, li.name, li.media_type
            ORDER BY play_count DESC
            LIMIT ?
        `, userID, fromMs, limit); err == nil {
			defer rows.Close()
			for rows.Next() {
				var ti UserTopItem
				var playCount int
				if err := rows.Scan(&ti.ItemID, &ti.Name, &ti.Type, &playCount); err == nil {
					ti.Hours = float64(playCount) // Using play count as a proxy for engagement
					detail.TopItems = append(detail.TopItems, ti)
				}
			}
		}

		// recent activity
		if rows, err := db.Query(`
            SELECT ps.started_at, li.id, li.name, li.media_type, 0.0 as pos_hours
            FROM play_sessions ps
            LEFT JOIN library_item li ON li.id = ps.item_id
            WHERE ps.user_id = ? AND ps.started_at >= ?
            AND (li.id IS NULL OR `+excludeLiveTvFilter()+`)
            ORDER BY ps.started_at DESC
            LIMIT ?
        `, userID, fromMs/1000, limit); err == nil {
			defer rows.Close()
			for rows.Next() {
				var a UserActivity
				if err := rows.Scan(&a.Timestamp, &a.ItemID, &a.ItemName, &a.ItemType, &a.PosHours); err == nil {
					detail.RecentActivity = append(detail.RecentActivity, a)
				}
			}
		}

		// Get total movies watched by user
		_ = db.QueryRow(`
			SELECT COUNT(DISTINCT ps.item_id)
			FROM play_sessions ps
			JOIN library_item li ON li.id = ps.item_id
			WHERE ps.user_id = ? AND (`+movieMediaPredicate("li")+`)
			AND ps.ended_at IS NOT NULL
		`, userID).Scan(&detail.TotalMovies)

		// Get total episodes watched by user
		_ = db.QueryRow(`
			SELECT COUNT(DISTINCT ps.item_id)
			FROM play_sessions ps
			JOIN library_item li ON li.id = ps.item_id
			WHERE ps.user_id = ? AND (`+episodeMediaPredicate("li")+`)
			AND ps.ended_at IS NOT NULL
		`, userID).Scan(&detail.TotalEpisodes)

		// Get total series finished - simplified approach without series_id
		// For now, we'll count unique series names that the user has watched episodes from
		_ = db.QueryRow(`
			SELECT COUNT(DISTINCT 
				CASE 
					WHEN li.name LIKE '%-%' THEN SUBSTR(li.name, 1, INSTR(li.name, ' - ') - 1)
					ELSE NULL
				END
			)
			FROM play_sessions ps
			JOIN library_item li ON li.id = ps.item_id
			WHERE ps.user_id = ? AND (`+episodeMediaPredicate("li")+`)
			AND ps.ended_at IS NOT NULL
			AND li.name LIKE '%-%'
		`, userID).Scan(&detail.TotalSeriesFinished)

		// Get last seen movies (limit 10)
		if rows, err := db.Query(`
			SELECT li.id, li.name, li.media_type, MAX(ps.ended_at) as last_seen
			FROM play_sessions ps
			JOIN library_item li ON li.id = ps.item_id
			WHERE ps.user_id = ? AND (`+movieMediaPredicate("li")+`)
			AND ps.ended_at IS NOT NULL
			GROUP BY li.id, li.name, li.media_type
			ORDER BY last_seen DESC
			LIMIT 10
		`, userID); err == nil {
			defer rows.Close()
			for rows.Next() {
				var movie UserTopItem
				var lastSeen int64
				if err := rows.Scan(&movie.ItemID, &movie.Name, &movie.Type, &lastSeen); err == nil {
					movie.Hours = float64(lastSeen) // Store timestamp for date display
					detail.LastSeenMovies = append(detail.LastSeenMovies, movie)
				}
			}
		}

		// Get last seen episodes (limit 10) and enrich display as "Series - Episode (SxxExx)"
		if rows, err := db.Query(`
            SELECT li.id, li.name, li.media_type, MAX(ps.ended_at) as last_seen
            FROM play_sessions ps
            JOIN library_item li ON li.id = ps.item_id
            WHERE ps.user_id = ? AND (`+episodeMediaPredicate("li")+`)
              AND ps.ended_at IS NOT NULL
            GROUP BY li.id, li.name, li.media_type
            ORDER BY last_seen DESC
            LIMIT 10
        `, userID); err == nil {
			defer rows.Close()
			tmp := make([]UserTopItem, 0, 10)
			ids := make([]string, 0, 10)
			lastSeenByID := make(map[string]int64)
			for rows.Next() {
				var it UserTopItem
				var lastSeen int64
				if err := rows.Scan(&it.ItemID, &it.Name, &it.Type, &lastSeen); err == nil {
					it.Hours = float64(lastSeen) // store timestamp for date display
					tmp = append(tmp, it)
					ids = append(ids, it.ItemID)
					lastSeenByID[it.ItemID] = lastSeen
				}
			}
			// Enrich via Emby for proper episode display
			if em != nil && len(ids) > 0 {
				if items, err := em.ItemsByIDs(ids); err == nil && len(items) > 0 {
					byID := make(map[string]emby.EmbyItem, len(items))
					for i := range items {
						byID[items[i].Id] = items[i]
					}
					detail.LastSeenEpisodes = make([]UserTopItem, 0, len(tmp))
					for _, it := range tmp {
						if emIt, ok := byID[it.ItemID]; ok && (emIt.Type == "Episode" || it.Type == "Episode") {
							name := emIt.Name
							series := emIt.SeriesName
							epcode := ""
							if emIt.ParentIndexNumber != nil && emIt.IndexNumber != nil {
								epcode = fmt.Sprintf("S%02dE%02d", *emIt.ParentIndexNumber, *emIt.IndexNumber)
							}
							disp := name
							if series != "" && name != "" {
								disp = fmt.Sprintf("%s - %s", series, name)
							} else if series != "" {
								disp = series
							}
							if epcode != "" {
								disp = disp + " (" + epcode + ")"
							}
							it.Name = disp
							it.Type = "Episode"
						}
						// ensure timestamp preserved
						if ts, ok := lastSeenByID[it.ItemID]; ok {
							it.Hours = float64(ts)
						}
						detail.LastSeenEpisodes = append(detail.LastSeenEpisodes, it)
					}
				} else {
					// fallback without enrichment
					detail.LastSeenEpisodes = append(detail.LastSeenEpisodes, tmp...)
				}
			} else {
				detail.LastSeenEpisodes = append(detail.LastSeenEpisodes, tmp...)
			}
		}

		// Get finished series list (limit 10) using series_id when available
		if rows, err := db.Query(`
            WITH watched AS (
                SELECT 
                    COALESCE(li.series_id, '') AS sid,
                    COALESCE(li.series_name, CASE WHEN INSTR(li.name,' - ')>0 THEN SUBSTR(li.name,1,INSTR(li.name,' - ')-1) ELSE li.name END) AS sname,
                    COUNT(DISTINCT li.name) AS watched_episodes,
                    MAX(ps.ended_at) AS last_watched
                FROM play_sessions ps
                JOIN library_item li ON li.id = ps.item_id
                WHERE ps.user_id = ? AND li.media_type='Episode' AND ps.ended_at IS NOT NULL
                GROUP BY sid, sname
            ), totals AS (
                SELECT 
                    COALESCE(series_id,'') AS sid,
                    COALESCE(series_name, CASE WHEN INSTR(name,' - ')>0 THEN SUBSTR(name,1,INSTR(name,' - ')-1) ELSE name END) AS sname,
                    COUNT(DISTINCT name) AS total_episodes
                FROM library_item
                WHERE media_type='Episode'
                GROUP BY sid, sname
            )
            SELECT watched.sid, watched.sname, 'Series' as media_type, watched.watched_episodes
            FROM watched 
            JOIN totals USING (sid, sname)
            WHERE watched.watched_episodes = totals.total_episodes AND totals.total_episodes > 1
            ORDER BY watched.last_watched DESC
            LIMIT 10
        `, userID); err == nil {
			defer rows.Close()
			for rows.Next() {
				var series UserTopItem
				var episodeCount int
				if err := rows.Scan(&series.ItemID, &series.Name, &series.Type, &episodeCount); err == nil {
					if strings.TrimSpace(series.ItemID) == "" && em != nil {
						if sid, _ := em.FindSeriesIDByName(series.Name); sid != "" {
							series.ItemID = sid
						}
					}
					series.Hours = float64(episodeCount)
					detail.FinishedSeries = append(detail.FinishedSeries, series)
				}
			}
		}

		return c.JSON(detail)
	}
}
