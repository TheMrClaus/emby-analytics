package stats

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"emby-analytics/internal/handlers/admin"

	"github.com/gofiber/fiber/v3"
)

type MoviesData struct {
	TotalMovies       int     `json:"total_movies"`
	LargestMovieGB    float64 `json:"largest_movie_gb"`
	LargestMovieName  string  `json:"largest_movie_name"`
	LongestRuntime    int     `json:"longest_runtime_minutes"`
	LongestMovieName  string  `json:"longest_movie_name"`
	ShortestRuntime   int     `json:"shortest_runtime_minutes"`
	ShortestMovieName string  `json:"shortest_movie_name"`
	NewestMovie       struct {
		Name string `json:"name"`
		Date string `json:"date"`
	} `json:"newest_movie"`
	MostWatchedMovie struct {
		Name  string  `json:"name"`
		Hours float64 `json:"hours"`
	} `json:"most_watched_movie"`
	TotalRuntimeHours    float64      `json:"total_runtime_hours"`
	PopularGenres        []GenreStats `json:"popular_genres"`
	MoviesAddedThisMonth int          `json:"movies_added_this_month"`
}

type GenreStats struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

func Movies(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		data := MoviesData{}

		serverType, serverID := normalizeServerParam(c.Query("server", ""))
		movieBase := "(" + movieMediaPredicate("") + ") AND " + excludeLiveTvFilter()
		movieWhere, movieArgs := appendServerFilter(movieBase, "", serverType, serverID)
		movieAliasBase := "(" + movieMediaPredicate("li") + ") AND " + excludeLiveTvFilterAlias("li")
		movieAliasWhere, movieAliasArgs := appendServerFilter(movieAliasBase, "li", serverType, serverID)

		// Count total movies (deduplicated by file_path for All Servers, item_id for single server)
		var countQuery string
		if serverType == "" && serverID == "" {
			// All Servers: deduplicate by file_path
			countQuery = fmt.Sprintf(`
				SELECT COUNT(DISTINCT %s)
				FROM library_item
				WHERE %s AND file_path IS NOT NULL AND file_path != ''`,
				normalizedFilePathExpr(""), movieWhere)
		} else {
			// Single server: use item_id
			countQuery = fmt.Sprintf(`
				SELECT COUNT(DISTINCT item_id)
				FROM library_item
				WHERE %s`, movieWhere)
		}
		err := db.QueryRow(countQuery, movieArgs...).Scan(&data.TotalMovies)
		if err != nil {
			log.Printf("[movies] Error counting movies: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count movies"})
		}

		// Get largest movie: prefer actual size, then bitrate*runtime, else heuristic
		largestQuery := fmt.Sprintf(`
            SELECT COALESCE(name, 'Unknown'),
                   COALESCE(
                     CASE WHEN file_size_bytes IS NOT NULL AND file_size_bytes > 0
                          THEN file_size_bytes / 1073741824.0
                     END,
                     CASE WHEN bitrate_bps > 0 AND run_time_ticks > 0
                          THEN (bitrate_bps * (run_time_ticks / 10000000.0) / 8.0) / 1073741824.0
                     END,
                     (COALESCE(run_time_ticks, 0) / 36000000000.0) * 
                     CASE 
                       WHEN height >= 2160 THEN 25.0
                       WHEN height >= 1080 THEN 8.0
                       WHEN height >= 720 THEN 4.0
                       ELSE 2.0
                     END
                   ) AS estimated_gb
            FROM library_item
            WHERE %s
            ORDER BY estimated_gb DESC
            LIMIT 1`, movieWhere)
		err = db.QueryRow(largestQuery, movieArgs...).Scan(&data.LargestMovieName, &data.LargestMovieGB)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding largest movie: %v", err)
		}

		// Get longest runtime movie, validating via live server metadata when available
		longestQuery := fmt.Sprintf(`
			SELECT id, server_id, server_type, item_id, name, run_time_ticks / 600000000, run_time_ticks
			FROM library_item 
			WHERE %s 
			  AND run_time_ticks > 0
			ORDER BY run_time_ticks DESC 
			LIMIT 1`, movieWhere)
		if candidate, cerr := findValidLongestMovie(db, longestQuery, movieArgs...); cerr == nil && candidate != nil {
			data.LongestMovieName = candidate.Name
			data.LongestRuntime = candidate.RuntimeMinutes
		} else if cerr != nil && cerr != sql.ErrNoRows {
			log.Printf("[movies] Error determining longest movie: %v", cerr)
		}

		// Get shortest runtime movie (but reasonable minimum of 30 minutes)
		shortestQuery := fmt.Sprintf(`
			SELECT name, run_time_ticks / 600000000 
			FROM library_item 
			WHERE %s 
			  AND run_time_ticks >= 18000000000
			ORDER BY run_time_ticks ASC 
			LIMIT 1`, movieWhere)
		err = db.QueryRow(shortestQuery, movieArgs...).Scan(&data.ShortestMovieName, &data.ShortestRuntime)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding shortest movie: %v", err)
		}

		// Get newest added movie
		newestQuery := fmt.Sprintf(`
			SELECT name, created_at 
			FROM library_item 
			WHERE %s
			ORDER BY created_at DESC 
			LIMIT 1`, movieWhere)
		err = db.QueryRow(newestQuery, movieArgs...).Scan(&data.NewestMovie.Name, &data.NewestMovie.Date)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding newest movie: %v", err)
		}

		// Get most watched movie using exact interval merging (avoids over/under-count)
		{
			// Candidate movie IDs that appear in intervals
			ids := []string{}
			intervalQuery := fmt.Sprintf(`
                SELECT DISTINCT pi.item_id
                FROM play_intervals pi
                JOIN library_item li ON li.id = pi.item_id
                WHERE %s
            `, movieAliasWhere)
			rows, qerr := db.Query(intervalQuery, movieAliasArgs...)
			if qerr == nil {
				defer rows.Close()
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err == nil && id != "" {
						ids = append(ids, id)
					}
				}
				_ = rows.Err()
			}

			if len(ids) > 0 {
				// Compute exact hours across "all time" window
				now := time.Now().UTC()
				hoursMap, herr := computeExactItemHours(db, ids, 0, now.AddDate(100, 0, 0).Unix())
				if herr == nil {
					var bestID string
					var bestHours float64
					for id, hrs := range hoursMap {
						if hrs > bestHours {
							bestHours = hrs
							bestID = id
						}
					}
					if bestID != "" {
						_ = db.QueryRow(`SELECT COALESCE(name,'Unknown') FROM library_item WHERE id = ?`, bestID).Scan(&data.MostWatchedMovie.Name)
						data.MostWatchedMovie.Hours = bestHours
					}
				} else {
					log.Printf("[movies] computeExactItemHours failed: %v", herr)
				}
			}
		}

		// Calculate total runtime hours
		totalRuntimeQuery := fmt.Sprintf(`
			SELECT COALESCE(SUM(run_time_ticks), 0) / 36000000000.0 
			FROM library_item 
			WHERE %s AND run_time_ticks > 0`, movieWhere)
		err = db.QueryRow(totalRuntimeQuery, movieArgs...).Scan(&data.TotalRuntimeHours)
		if err != nil {
			log.Printf("[movies] Error calculating total runtime: %v", err)
		}

		// Popular genres (top 5) by any-genre token.
		// Split comma-separated genres per item and count distinct items per token.
		{
			var cnt int
			row := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('library_item') WHERE name = 'genres'`)
			if err := row.Scan(&cnt); err == nil && cnt > 0 {
				genreQuery := fmt.Sprintf(`
                WITH RECURSIVE base AS (
                  SELECT id, REPLACE(genres, ', ', ',') AS g
                  FROM library_item
                  WHERE %s AND genres IS NOT NULL AND genres != ''
                ),
                split(id, token, rest) AS (
				  SELECT id,
				         TRIM(CASE WHEN INSTR(g, ',') = 0 THEN g ELSE SUBSTR(g, 1, INSTR(g, ',') - 1) END),
				         TRIM(CASE WHEN INSTR(g, ',') = 0 THEN '' ELSE SUBSTR(g, INSTR(g, ',') + 1) END)
				  FROM base
				  UNION ALL
				  SELECT id,
				         TRIM(CASE WHEN INSTR(rest, ',') = 0 THEN rest ELSE SUBSTR(rest, 1, INSTR(rest, ',') - 1) END),
				         TRIM(CASE WHEN INSTR(rest, ',') = 0 THEN '' ELSE SUBSTR(rest, INSTR(rest, ',') + 1) END)
				  FROM split
				  WHERE rest <> ''
				)
				SELECT token AS genre, COUNT(DISTINCT id) AS count
				FROM split
				WHERE token IS NOT NULL AND token != ''
				GROUP BY token
				ORDER BY count DESC
				LIMIT 5`, movieWhere)
				genreArgs := append([]interface{}{}, movieArgs...)
				if rows, err := db.Query(genreQuery, genreArgs...); err == nil {
					defer rows.Close()
					for rows.Next() {
						var g GenreStats
						if err := rows.Scan(&g.Genre, &g.Count); err == nil {
							data.PopularGenres = append(data.PopularGenres, g)
						}
					}
				} else {
					log.Printf("[movies] Error fetching popular genres: %v", err)
				}
			} else {
				log.Printf("[movies] 'genres' column not found; skipping popular genres")
			}
		}

		// Get movies added this month
		addedQuery := fmt.Sprintf(`
			SELECT COUNT(*) 
			FROM library_item 
			WHERE %s AND created_at >= date('now', 'start of month')`, movieWhere)
		err = db.QueryRow(addedQuery, movieArgs...).Scan(&data.MoviesAddedThisMonth)
		if err != nil {
			log.Printf("[movies] Error counting movies added this month: %v", err)
		}

		duration := time.Since(start)
		isSlowQuery := duration > 1*time.Second
		if isSlowQuery {
			log.Printf("[movies] WARNING: Slow query took %v", duration)
		}

		// Track metrics
		admin.IncrementQueryMetrics(duration, isSlowQuery)

		log.Printf("[movies] Successfully fetched data in %v: total=%d, newest=%s, most_watched=%s",
			duration, data.TotalMovies, data.NewestMovie.Name, data.MostWatchedMovie.Name)

		return c.JSON(data)
	}
}

func clearRuntimeOutlier(db *sql.DB, libraryID string, runtimeMinutes int) error {
	if strings.TrimSpace(libraryID) == "" {
		return fmt.Errorf("empty library item id")
	}
	_, err := db.Exec(`
		UPDATE library_item
		SET run_time_ticks = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, libraryID)
	if err == nil {
		log.Printf("[movies] Cleared unrealistic runtime (%d minutes) for library item %s", runtimeMinutes, libraryID)
	}
	return err
}

type movieRuntimeCandidate struct {
	LibraryID      string
	ServerID       string
	ServerType     string
	ItemID         string
	Name           string
	RuntimeMinutes int
	RuntimeTicks   int64
}

func findValidLongestMovie(db *sql.DB, query string, args ...any) (*movieRuntimeCandidate, error) {
	var lastErr error
	for attempt := 0; attempt < runtimeOutlierMaxFixPasses; attempt++ {
		candidate := movieRuntimeCandidate{}
		err := db.QueryRow(query, args...).Scan(&candidate.LibraryID, &candidate.ServerID, &candidate.ServerType, &candidate.ItemID, &candidate.Name, &candidate.RuntimeMinutes, &candidate.RuntimeTicks)
		if err != nil {
			return nil, err
		}
		if candidate.RuntimeMinutes <= 0 {
			return &candidate, nil
		}
		if !isRuntimeOutlier(candidate.RuntimeMinutes) {
			return &candidate, nil
		}
		if reconcileMovieRuntimeWithServer(db, &candidate) {
			if !isRuntimeOutlier(candidate.RuntimeMinutes) {
				return &candidate, nil
			}
		} else {
			lastErr = fmt.Errorf("runtime still implausible after reconciliation")
		}
		if err := clearRuntimeOutlier(db, candidate.LibraryID, candidate.RuntimeMinutes); err != nil {
			return nil, err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, sql.ErrNoRows
}

func reconcileMovieRuntimeWithServer(db *sql.DB, cand *movieRuntimeCandidate) bool {
	mgr := getMultiServerManager()
	if mgr == nil || cand == nil {
		return false
	}
	client, ok := mgr.GetClient(strings.TrimSpace(cand.ServerID))
	if !ok || client == nil {
		return false
	}
	if strings.TrimSpace(cand.ItemID) == "" {
		return false
	}
	items, err := client.ItemsByIDs([]string{cand.ItemID})
	if err != nil || len(items) == 0 {
		return false
	}
	remote := items[0]
	if remote.RuntimeMs == nil || *remote.RuntimeMs <= 0 {
		return false
	}
	minutes := int((*remote.RuntimeMs + 59_999) / 60_000)
	if minutes <= 0 {
		return false
	}
	// Update local cache with authoritative runtime from server
	ticks := *remote.RuntimeMs * 10_000
	if _, err := db.Exec(`
		UPDATE library_item
		SET run_time_ticks = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, ticks, cand.LibraryID); err != nil {
		log.Printf("[movies] Failed to upsert reconciled runtime for %s: %v", cand.LibraryID, err)
	}
	cand.RuntimeMinutes = minutes
	cand.RuntimeTicks = ticks
	return true
}
