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

		// Count total movies
		err := db.QueryRow(fmt.Sprintf(`
			SELECT COUNT(*) 
			FROM library_item 
			WHERE %s`, movieWhere), movieArgs...).Scan(&data.TotalMovies)
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

		// Get longest runtime movie
		longestQuery := fmt.Sprintf(`
			SELECT id, name, run_time_ticks / 600000000 
			FROM library_item 
			WHERE %s 
			  AND run_time_ticks > 0
			ORDER BY run_time_ticks DESC 
			LIMIT 1`, movieWhere)
		{
			var longestID string
			for attempt := 0; attempt < runtimeOutlierMaxFixPasses; attempt++ {
				err = db.QueryRow(longestQuery, movieArgs...).Scan(&longestID, &data.LongestMovieName, &data.LongestRuntime)
				if err != nil {
					if err != sql.ErrNoRows {
						log.Printf("[movies] Error finding longest movie: %v", err)
					}
					break
				}
				if !isRuntimeOutlier(data.LongestRuntime) {
					break
				}
				if strings.TrimSpace(longestID) == "" {
					break
				}
				if ferr := clearRuntimeOutlier(db, longestID, data.LongestRuntime); ferr != nil {
					log.Printf("[movies] Failed clearing runtime outlier for %s: %v", longestID, ferr)
					break
				}
				// Retry to find the next suitable candidate after clearing the outlier
				longestID = ""
				data.LongestMovieName = ""
				data.LongestRuntime = 0
			}
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
