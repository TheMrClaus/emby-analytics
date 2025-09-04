package stats

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"fmt"
	"log"
	"strconv"
	"strings"

	emby "emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

// normalize maps various casings/variants into our 4 buckets.
func normalize(method string) string {
	m := strings.ToLower(strings.TrimSpace(method))
	switch m {
	case "directplay", "direct play":
		return "DirectPlay"
	case "directstream", "direct stream":
		return "DirectStream"
	case "transcode", "transcoding", "h264", "h265", "hevc":
		return "Transcode"
	case "direct": // handles the "Direct" value from Emby
		return "DirectPlay"
	case "remux", "copy", "directcopy":
		return "DirectStream"
	case "convert", "encoding":
		return "Transcode"
	case "":
		return "Unknown"
	default:
		return "Unknown"
	}
}

// PlayMethods returns a breakdown of playback methods over the last N days (default 30).
type SessionDetail struct {
	ItemName          string `json:"item_name"`
	ItemType          string `json:"item_type"`
	ItemID            string `json:"item_id"`
	DeviceID          string `json:"device_id"`
	DeviceName        string `json:"device_name"`
	ClientName        string `json:"client_name"`
	VideoMethod       string `json:"video_method"`
	AudioMethod       string `json:"audio_method"`
	SubtitleTranscode bool   `json:"subtitle_transcode"`
	UserID            string `json:"user_id"`
	UserName          string `json:"user_name"`
	StartedAt         int64  `json:"started_at"`
	EndedAt           *int64 `json:"ended_at"`
	SessionID         string `json:"session_id"`
	PlayMethod        string `json:"play_method"`
}

func PlayMethods(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fiber v3: parse query params manually
		daysStr := c.Query("days", "30")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			days = 30
		}

		// Parse pagination parameters
		limitStr := c.Query("limit", "50")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 1000 {
			limit = 50
		}

		offsetStr := c.Query("offset", "0")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}

		// Parse filter parameters
		showAll := c.Query("show_all", "false") == "true"
		userFilter := c.Query("user_id", "")
		mediaTypeFilter := c.Query("media_type", "")

		// Check if enhanced columns exist by checking table structure
		var hasVideoMethod bool
		row := db.QueryRow(`
			SELECT COUNT(*) 
			FROM pragma_table_info('play_sessions') 
			WHERE name = 'video_method'
		`)
		var count int
		if err := row.Scan(&count); err == nil && count > 0 {
			hasVideoMethod = true
		}

		if !hasVideoMethod {
			log.Printf("[PlayMethods] Enhanced columns not found, using legacy mode")
			return legacyPlayMethods(c, db, days, limit, offset)
		}

		// Enhanced query with new columns - handle empty strings and NULLs properly
		query := `
            WITH derived AS (
                SELECT 
                    -- Per-stream derivation (no blanket fallback)
                    CASE 
                        WHEN lower(COALESCE(video_method,'')) = 'transcode' THEN 'Transcode'
                        WHEN COALESCE(video_codec_from,'') <> '' AND COALESCE(video_codec_to,'') <> '' 
                            AND lower(video_codec_from) <> lower(video_codec_to) THEN 'Transcode'
                        WHEN (
                            instr(lower(COALESCE(transcode_reasons,'')), 'subtitle') > 0 OR 
                            instr(lower(COALESCE(transcode_reasons,'')), 'burn') > 0 OR 
                            instr(lower(COALESCE(transcode_reasons,'')), 'video') > 0
                        ) THEN 'Transcode'
                        ELSE 'DirectPlay'
                    END AS video_method,
                    CASE 
                        WHEN lower(COALESCE(audio_method,'')) = 'transcode' THEN 'Transcode'
                        WHEN COALESCE(audio_codec_from,'') <> '' AND COALESCE(audio_codec_to,'') <> '' 
                            AND lower(audio_codec_from) <> lower(audio_codec_to) THEN 'Transcode'
                        WHEN instr(lower(COALESCE(transcode_reasons,'')), 'audio') > 0 THEN 'Transcode'
                        ELSE 'DirectPlay'
                    END AS audio_method,
                    play_method
                FROM play_sessions
                WHERE started_at >= (strftime('%s','now') - (? * 86400))
                    AND started_at IS NOT NULL
            )
            SELECT 
                video_method,
                audio_method,
                CASE WHEN play_method = 'Transcode' OR video_method = 'Transcode' OR audio_method = 'Transcode' THEN 'Transcode' ELSE 'DirectPlay' END AS overall_method,
                COUNT(*) AS cnt
            FROM derived
            GROUP BY 1, 2, 3
        `

		// Build session query with filters
		sessionQueryBase := `
            SELECT 
                ps.item_name, 
                ps.item_type, 
                ps.device_id,
                COALESCE(ps.device_id, 'Unknown Device') as device_name,
                ps.client_name, 
                ps.item_id, 
                ps.user_id,
                COALESCE(eu.name, ps.user_id) as user_name,
                ps.started_at,
                ps.ended_at,
                ps.session_id,
                ps.play_method,
                -- Derive consistent methods for session details
                CASE 
                    WHEN lower(COALESCE(ps.video_method,'')) = 'transcode' THEN 'Transcode'
                    WHEN COALESCE(ps.video_codec_from,'') <> '' AND COALESCE(ps.video_codec_to,'') <> '' 
                        AND lower(ps.video_codec_from) <> lower(ps.video_codec_to) THEN 'Transcode'
                    WHEN (
                        instr(lower(COALESCE(ps.transcode_reasons,'')), 'subtitle') > 0 OR 
                        instr(lower(COALESCE(ps.transcode_reasons,'')), 'burn') > 0 OR 
                        instr(lower(COALESCE(ps.transcode_reasons,'')), 'video') > 0
                    ) THEN 'Transcode'
                    ELSE 'DirectPlay'
                END AS video_method,
                CASE 
                    WHEN lower(COALESCE(ps.audio_method,'')) = 'transcode' THEN 'Transcode'
                    WHEN COALESCE(ps.audio_codec_from,'') <> '' AND COALESCE(ps.audio_codec_to,'') <> '' 
                        AND lower(ps.audio_codec_from) <> lower(ps.audio_codec_to) THEN 'Transcode'
                    WHEN instr(lower(COALESCE(ps.transcode_reasons,'')), 'audio') > 0 THEN 'Transcode'
                    ELSE 'DirectPlay'
                END AS audio_method,
                -- Per-session subtitle transcoding detection
                CASE 
                    WHEN instr(lower(COALESCE(ps.transcode_reasons,'')), 'subtitle') > 0 OR 
                         instr(lower(COALESCE(ps.transcode_reasons,'')), 'burn') > 0 THEN 1
                    ELSE 0
                END AS subtitle_transcode
            FROM play_sessions ps
            LEFT JOIN emby_user eu ON ps.user_id = eu.id
            WHERE ps.started_at >= (strftime('%s','now') - (? * 86400))
                AND ps.started_at IS NOT NULL`

		// Add filters
		var queryParams []interface{}
		queryParams = append(queryParams, days)

		if !showAll {
			// Only show transcoding sessions when show_all is false (backward compatibility)
			sessionQueryBase += ` AND (
                ps.play_method = 'Transcode' OR
                lower(COALESCE(ps.video_method,'')) = 'transcode' OR
                lower(COALESCE(ps.audio_method,'')) = 'transcode' OR
                (COALESCE(ps.video_codec_from,'') <> '' AND COALESCE(ps.video_codec_to,'') <> '' 
                 AND lower(ps.video_codec_from) <> lower(ps.video_codec_to)) OR
                (COALESCE(ps.audio_codec_from,'') <> '' AND COALESCE(ps.audio_codec_to,'') <> '' 
                 AND lower(ps.audio_codec_from) <> lower(ps.audio_codec_to)) OR
                (
                    instr(lower(COALESCE(ps.transcode_reasons,'')), 'subtitle') > 0 OR 
                    instr(lower(COALESCE(ps.transcode_reasons,'')), 'burn') > 0 OR 
                    instr(lower(COALESCE(ps.transcode_reasons,'')), 'video') > 0 OR
                    instr(lower(COALESCE(ps.transcode_reasons,'')), 'audio') > 0
                )
            ) AND (
                -- Exclude sessions where ALL streams are direct (truly direct sessions)
                NOT (
                    lower(COALESCE(ps.video_method,'')) <> 'transcode' AND
                    lower(COALESCE(ps.audio_method,'')) <> 'transcode' AND
                    NOT (instr(lower(COALESCE(ps.transcode_reasons,'')), 'subtitle') > 0 OR 
                         instr(lower(COALESCE(ps.transcode_reasons,'')), 'burn') > 0) AND
                    NOT (
                        (COALESCE(ps.video_codec_from,'') <> '' AND COALESCE(ps.video_codec_to,'') <> '' 
                         AND lower(ps.video_codec_from) <> lower(ps.video_codec_to)) OR
                        (COALESCE(ps.audio_codec_from,'') <> '' AND COALESCE(ps.audio_codec_to,'') <> '' 
                         AND lower(ps.audio_codec_from) <> lower(ps.audio_codec_to))
                    )
                )
            )`
		}

		if userFilter != "" {
			sessionQueryBase += " AND ps.user_id = ?"
			queryParams = append(queryParams, userFilter)
		}

		if mediaTypeFilter != "" {
			sessionQueryBase += " AND ps.item_type = ?"
			queryParams = append(queryParams, mediaTypeFilter)
		}

		sessionQuery := sessionQueryBase + `
            ORDER BY ps.started_at DESC
            LIMIT ? OFFSET ?
        `
		queryParams = append(queryParams, limit, offset)

		rows, err := db.Query(query, days)
		if err != nil {
			logging.Debug("Enhanced query failed: %v", err)
			return legacyPlayMethods(c, db, days, limit, offset)
		}
		defer rows.Close()

		// Detailed breakdown
		methodBreakdown := make(map[string]int)

		// Simplified summary: only DirectPlay vs Transcode
		summary := map[string]int{
			"DirectPlay": 0,
			"Transcode":  0,
		}

		// Detailed breakdown for transcode subcategories
		transcodeDetails := map[string]int{
			"TranscodeVideo":    0,
			"TranscodeAudio":    0,
			"TranscodeSubtitle": 0,
		}

		// Store session details for frontend
		var sessionDetails []SessionDetail

		// Process results with proper variable declarations
		for rows.Next() {
			var videoMethod, audioMethod, overallMethod string
			var cnt int

			if err := rows.Scan(&videoMethod, &audioMethod, &overallMethod, &cnt); err != nil {
				logging.Debug("Scan error: %v", err)
				continue
			}

			// Normalize the methods to handle variations
			normalizedVideo := normalize(videoMethod)
			normalizedAudio := normalize(audioMethod)

			// Create detailed key with normalized values
			key := fmt.Sprintf("%s|%s", normalizedVideo, normalizedAudio)
			methodBreakdown[key] = cnt

			// Update variables for categorization logic
			videoMethod = normalizedVideo
			audioMethod = normalizedAudio

			// We now use overallMethod returned from SQL to decide summary buckets
			// but still count per-stream details for the bubbles.
			if strings.EqualFold(overallMethod, "Transcode") {
				summary["Transcode"] += cnt
			} else {
				summary["DirectPlay"] += cnt
			}

			// Track detailed transcode reasons (per-stream)
			if videoMethod == "Transcode" {
				transcodeDetails["TranscodeVideo"] += cnt
			}
			if audioMethod == "Transcode" {
				transcodeDetails["TranscodeAudio"] += cnt
			}
			// Subtitle transcoding detection using transcode_reasons field
			// This is handled in the SQL query, but we could add additional logic here if needed
		}

		if err := rows.Err(); err != nil {
			logging.Debug("Rows error: %v", err)
			return legacyPlayMethods(c, db, days, limit, offset)
		}

		// Fetch detailed session information
		sessionRows, sessionErr := db.Query(sessionQuery, queryParams...)
		if sessionErr != nil {
			logging.Debug("Session query failed: %v", sessionErr)
		} else {
			defer sessionRows.Close()
			for sessionRows.Next() {
				var session SessionDetail
				var subtitleTranscodeInt int
				if err := sessionRows.Scan(
					&session.ItemName, &session.ItemType, &session.DeviceID, &session.DeviceName,
					&session.ClientName, &session.ItemID, &session.UserID, &session.UserName,
					&session.StartedAt, &session.EndedAt, &session.SessionID, &session.PlayMethod,
					&session.VideoMethod, &session.AudioMethod, &subtitleTranscodeInt); err != nil {
					logging.Debug("Session scan error: %v", err)
					continue
				}
				session.SubtitleTranscode = subtitleTranscodeInt == 1
				sessionDetails = append(sessionDetails, session)
			}
		}

		// Count subtitle transcoding from transcode_reasons field
		subtitleQuery := `
            SELECT COUNT(*) FROM play_sessions
            WHERE started_at >= (strftime('%s','now') - (? * 86400))
                AND started_at IS NOT NULL
                AND (
                    instr(lower(COALESCE(transcode_reasons,'')), 'subtitle') > 0 OR 
                    instr(lower(COALESCE(transcode_reasons,'')), 'burn') > 0
                )
        `
		var subtitleCount int
		if err := db.QueryRow(subtitleQuery, days).Scan(&subtitleCount); err == nil {
			transcodeDetails["TranscodeSubtitle"] = subtitleCount
		}

		// Calculate truly direct sessions (all streams direct)
		directQuery := `
            SELECT COUNT(*) FROM play_sessions ps
            WHERE ps.started_at >= (strftime('%s','now') - (? * 86400))
                AND ps.started_at IS NOT NULL
                AND (
                    lower(COALESCE(ps.video_method,'')) <> 'transcode' AND
                    lower(COALESCE(ps.audio_method,'')) <> 'transcode' AND
                    NOT (instr(lower(COALESCE(ps.transcode_reasons,'')), 'subtitle') > 0 OR 
                         instr(lower(COALESCE(ps.transcode_reasons,'')), 'burn') > 0)
                )
                AND NOT (
                    (COALESCE(ps.video_codec_from,'') <> '' AND COALESCE(ps.video_codec_to,'') <> '' 
                     AND lower(ps.video_codec_from) <> lower(ps.video_codec_to)) OR
                    (COALESCE(ps.audio_codec_from,'') <> '' AND COALESCE(ps.audio_codec_to,'') <> '' 
                     AND lower(ps.audio_codec_from) <> lower(ps.audio_codec_to))
                )
        `
		var directCount int
		if err := db.QueryRow(directQuery, days).Scan(&directCount); err == nil {
			transcodeDetails["Direct"] = directCount
		}

		// Enrich item names for episodes: "Series - Episode (SxxExx)"
		sessionDetails = enrichSessionDetails(sessionDetails, em)

		// Ensure we have the basic methods even if not in data
		if summary["DirectPlay"] == 0 && summary["Transcode"] == 0 {
			// If no data, try legacy mode as fallback
			return legacyPlayMethods(c, db, days, limit, offset)
		}

		return c.JSON(fiber.Map{
			"methods":          summary,
			"detailed":         methodBreakdown,
			"transcodeDetails": transcodeDetails,
			"sessionDetails":   sessionDetails,
			"days":             days,
			"pagination": fiber.Map{
				"limit":  limit,
				"offset": offset,
				"count":  len(sessionDetails),
			},
		})
	}
}

// legacyPlayMethods provides the original functionality when new columns don't exist
func legacyPlayMethods(c fiber.Ctx, db *sql.DB, days int, limit int, offset int) error {
	query := `
		SELECT
			COALESCE(play_method, '') AS raw_method,
			COUNT(*) AS cnt
		FROM play_sessions
		WHERE started_at >= (strftime('%s','now') - (? * 86400))
			AND started_at IS NOT NULL
		GROUP BY raw_method
	`

	rows, err := db.Query(query, days)
	if err != nil {
		logging.Debug("Legacy query failed: %v", err)
		// Return empty data instead of error
		return c.JSON(fiber.Map{
			"methods": map[string]int{
				"DirectPlay":   0,
				"DirectStream": 0,
				"Transcode":    0,
				"Unknown":      0,
			},
			"detailed": make(map[string]int),
			"days":     days,
			"pagination": fiber.Map{
				"limit":  limit,
				"offset": offset,
				"count":  0,
			},
		})
	}
	defer rows.Close()

	// Simplified summary for legacy mode
	summary := map[string]int{
		"DirectPlay": 0,
		"Transcode":  0,
	}

	transcodeDetails := map[string]int{
		"TranscodeVideo":    0,
		"TranscodeAudio":    0,
		"TranscodeSubtitle": 0,
	}

	for rows.Next() {
		var raw string
		var cnt int
		if err := rows.Scan(&raw, &cnt); err != nil {
			logging.Debug("Legacy scan error: %v", err)
			continue
		}
		normalized := normalize(raw)
		if normalized == "DirectPlay" {
			summary["DirectPlay"] += cnt
		} else {
			summary["Transcode"] += cnt
			// In legacy mode, we can't distinguish video/audio, so add to both
			transcodeDetails["TranscodeVideo"] += cnt
			transcodeDetails["TranscodeAudio"] += cnt
		}
	}

	if err := rows.Err(); err != nil {
		logging.Debug("Legacy rows error: %v", err)
	}

	// Return in the format expected by frontend
	return c.JSON(fiber.Map{
		"methods":          summary,
		"detailed":         make(map[string]int),
		"transcodeDetails": transcodeDetails,
		"sessionDetails":   []interface{}{}, // empty for legacy mode
		"days":             days,
		"pagination": fiber.Map{
			"limit":  limit,
			"offset": offset,
			"count":  0, // no sessions in legacy mode
		},
	})
}

// enrichSessionDetails updates ItemName for episodes to "Series - Episode (SxxExx)" and movies to "Movie (year)"
func enrichSessionDetails(details []SessionDetail, em *emby.Client) []SessionDetail {
	if em == nil || len(details) == 0 {
		return details
	}
	// Collect unique item IDs
	idSet := make(map[string]struct{})
	for _, d := range details {
		if d.ItemID != "" {
			idSet[d.ItemID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return details
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	items, err := em.ItemsByIDs(ids)
	if err != nil {
		// Best effort; return unmodified on error
		return details
	}
	byID := make(map[string]*emby.EmbyItem, len(items))
	for i := range items {
		it := items[i]
		byID[it.Id] = &it
	}
	for i := range details {
		d := &details[i]
		it, ok := byID[d.ItemID]
		if !ok || it == nil {
			continue
		}
		// Normalize type if missing
		if d.ItemType == "" && it.Type != "" {
			d.ItemType = it.Type
		}
		if strings.EqualFold(it.Type, "Episode") {
			// Use canonical episode name: "Series - Episode (SxxExx)"
			epTitle := it.Name
			series := it.SeriesName
			epcode := ""
			if it.ParentIndexNumber != nil && it.IndexNumber != nil {
				epcode = fmt.Sprintf("S%02dE%02d", *it.ParentIndexNumber, *it.IndexNumber)
			}
			if series != "" && epTitle != "" && epcode != "" {
				d.ItemName = fmt.Sprintf("%s - %s (%s)", series, epTitle, epcode)
			} else if series != "" && epTitle != "" {
				d.ItemName = fmt.Sprintf("%s - %s", series, epTitle)
			} else if epTitle != "" {
				d.ItemName = epTitle
			}
			if d.ItemType == "" {
				d.ItemType = "Episode"
			}
		} else if strings.EqualFold(it.Type, "Movie") {
			// Format movies as "Movie (year)"
			movieTitle := it.Name
			if movieTitle != "" {
				if it.ProductionYear != nil && *it.ProductionYear > 0 {
					d.ItemName = fmt.Sprintf("%s (%d)", movieTitle, *it.ProductionYear)
				} else {
					d.ItemName = movieTitle
				}
			}
			if d.ItemType == "" {
				d.ItemType = "Movie"
			}
		} else if it.Name != "" {
			d.ItemName = it.Name
			if d.ItemType == "" && it.Type != "" {
				d.ItemType = it.Type
			}
		}
	}
	return details
}
