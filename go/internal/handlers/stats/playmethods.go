package stats

import (
    "database/sql"
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
    ItemName    string `json:"item_name"`
    ItemType    string `json:"item_type"`
    ItemID      string `json:"item_id"`
    DeviceID    string `json:"device_id"`
    ClientName  string `json:"client_name"`
    VideoMethod string `json:"video_method"`
    AudioMethod string `json:"audio_method"`
}

func PlayMethods(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fiber v3: parse query param manually
		daysStr := c.Query("days", "30")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			days = 30
		}

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
			return legacyPlayMethods(c, db, days)
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

		// Query for detailed sessions (only transcoding sessions)
        // Select recent sessions where either stream actually transcodes (derived),
        // regardless of the raw top-level play_method value.
        sessionQuery := `
            SELECT item_name, item_type, device_id, client_name, item_id, video_method, audio_method
            FROM (
                SELECT 
                    item_name, item_type, device_id, client_name, item_id, started_at, play_method,
                    -- Derive consistent methods for session details
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
                    END AS audio_method
                FROM play_sessions 
                WHERE started_at >= (strftime('%s','now') - (? * 86400))
                    AND started_at IS NOT NULL
            )
            WHERE video_method = 'Transcode' OR audio_method = 'Transcode' OR play_method = 'Transcode'
            ORDER BY started_at DESC
            LIMIT 50
        `

		rows, err := db.Query(query, days)
		if err != nil {
			log.Printf("[PlayMethods] Enhanced query failed: %v", err)
			return legacyPlayMethods(c, db, days)
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
                log.Printf("[PlayMethods] Scan error: %v", err)
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
            // TODO: Add subtitle transcoding detection when available
		}

		if err := rows.Err(); err != nil {
			log.Printf("[PlayMethods] Rows error: %v", err)
			return legacyPlayMethods(c, db, days)
		}

		// Fetch detailed session information for transcoding sessions
		sessionRows, sessionErr := db.Query(sessionQuery, days)
		if sessionErr != nil {
			log.Printf("[PlayMethods] Session query failed: %v", sessionErr)
		} else {
			defer sessionRows.Close()
			for sessionRows.Next() {
				var session SessionDetail
				if err := sessionRows.Scan(&session.ItemName, &session.ItemType, &session.DeviceID,
					&session.ClientName, &session.ItemID, &session.VideoMethod, &session.AudioMethod); err != nil {
					log.Printf("[PlayMethods] Session scan error: %v", err)
					continue
				}
				sessionDetails = append(sessionDetails, session)
			}
		}

        // Enrich item names for episodes: "Series - Episode (SxxExx)"
        sessionDetails = enrichSessionDetails(sessionDetails, em)

		// Ensure we have the basic methods even if not in data
		if summary["DirectPlay"] == 0 && summary["Transcode"] == 0 {
			// If no data, try legacy mode as fallback
			return legacyPlayMethods(c, db, days)
		}

		return c.JSON(fiber.Map{
			"methods":          summary,
			"detailed":         methodBreakdown,
			"transcodeDetails": transcodeDetails,
			"sessionDetails":   sessionDetails,
			"days":             days,
		})
	}
}

// legacyPlayMethods provides the original functionality when new columns don't exist
func legacyPlayMethods(c fiber.Ctx, db *sql.DB, days int) error {
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
		log.Printf("[PlayMethods] Legacy query failed: %v", err)
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
			log.Printf("[PlayMethods] Legacy scan error: %v", err)
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
		log.Printf("[PlayMethods] Legacy rows error: %v", err)
	}

	// Return in the format expected by frontend
	return c.JSON(fiber.Map{
		"methods":          summary,
		"detailed":         make(map[string]int),
		"transcodeDetails": transcodeDetails,
		"sessionDetails":   []interface{}{}, // empty for legacy mode
		"days":             days,
	})
}

// enrichSessionDetails updates ItemName for episodes to "Series - Episode (SxxExx)"
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
            // Use canonical episode name
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
        } else if it.Name != "" {
            d.ItemName = it.Name
            if d.ItemType == "" && it.Type != "" {
                d.ItemType = it.Type
            }
        }
    }
    return details
}
