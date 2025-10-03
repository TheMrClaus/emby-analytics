package stats

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"emby-analytics/internal/handlers/admin"

	"github.com/gofiber/fiber/v3"
)

// SeriesData holds aggregated stats across TV series/episodes.
type SeriesData struct {
	TotalSeries   int `json:"total_series"`
	TotalEpisodes int `json:"total_episodes"`

	LargestSeriesName string  `json:"largest_series_name"`
	LargestSeriesGB   float64 `json:"largest_series_total_gb"`

	LargestEpisodeName string  `json:"largest_episode_name"`
	LargestEpisodeGB   float64 `json:"largest_episode_gb"`

	LongestSeriesName    string `json:"longest_series_name"`
	LongestSeriesMinutes int    `json:"longest_series_runtime_minutes"`

	MostWatchedSeries struct {
		Name  string  `json:"name"`
		Hours float64 `json:"hours"`
	} `json:"most_watched_series"`

	TotalEpisodeRuntimeHours float64 `json:"total_episode_runtime_hours"`

	NewestSeries struct {
		Name string `json:"name"`
		Date string `json:"date"`
	} `json:"newest_series"`

	EpisodesAddedThisMonth int          `json:"episodes_added_this_month"`
	PopularGenres          []GenreStats `json:"popular_genres"`
}

// helper SQL fragment to derive a series name from the episode's display name convention
// We assume episodes have name like "Series Name - Episode Name (SxxExx)" after enrichment,
// otherwise this returns NULL/empty.
const seriesNameExpr = "TRIM(CASE WHEN INSTR(name, ' - ') > 0 THEN SUBSTR(name, 1, INSTR(name, ' - ') - 1) ELSE NULL END)"
const seriesResolvedNameExpr = "COALESCE(NULLIF(TRIM(COALESCE(series_name, '')), ''), " + seriesNameExpr + ")"
const seriesKeyExpr = "COALESCE(NULLIF(TRIM(COALESCE(series_id, '')), ''), " + seriesResolvedNameExpr + ")"

func Series(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		data := SeriesData{}
		var err error

		serverType, serverID := normalizeServerParam(c.Query("server", ""))
		episodeBase := "(" + episodeMediaPredicate("") + ") AND " + excludeLiveTvFilter()
		episodeWhere, episodeArgs := appendServerFilter(episodeBase, "", serverType, serverID)
		episodeAliasBase := "(" + episodeMediaPredicate("li") + ") AND " + excludeLiveTvFilterAlias("li")
		episodeAliasWhere, episodeAliasArgs := appendServerFilter(episodeAliasBase, "li", serverType, serverID)

		// Total series: prefer 'series' table if populated; fallback to derived from episodes
		var seriesTableCount int
		if serverType == "" && serverID == "" {
			if e := db.QueryRow(`SELECT COUNT(*) FROM series`).Scan(&seriesTableCount); e == nil && seriesTableCount > 0 {
				data.TotalSeries = seriesTableCount
			} else {
				seriesTableCount = 0
			}
		}
		if data.TotalSeries == 0 {
			countQuery := fmt.Sprintf(`
                SELECT COUNT(*) FROM (
                    SELECT DISTINCT %s AS series_key
                    FROM library_item
                    WHERE %s
                ) WHERE series_key IS NOT NULL AND series_key != ''
            `, seriesKeyExpr, episodeWhere)
			err = db.QueryRow(countQuery, episodeArgs...).Scan(&data.TotalSeries)
			if err != nil {
				log.Printf("[series] Error counting series: %v", err)
				return c.Status(500).JSON(fiber.Map{"error": "Failed to count series"})
			}
		}

		// Total episodes (deduplicated by file_path for All Servers, item_id for single server)
		var episodeCountQuery string
		if serverType == "" && serverID == "" {
			// All Servers: deduplicate by file_path
			episodeCountQuery = fmt.Sprintf(`
				SELECT COUNT(DISTINCT %s)
				FROM library_item
				WHERE %s AND file_path IS NOT NULL AND file_path != ''`,
				normalizedFilePathExpr(""), episodeWhere)
		} else {
			// Single server: use item_id
			episodeCountQuery = fmt.Sprintf(`
				SELECT COUNT(DISTINCT item_id)
				FROM library_item
				WHERE %s`, episodeWhere)
		}
		err = db.QueryRow(episodeCountQuery, episodeArgs...).Scan(&data.TotalEpisodes)
		if err != nil {
			log.Printf("[series] Error counting episodes: %v", err)
		}

		// Largest TV series by total size across episodes (prefer file size, then bitrate*runtime, else heuristic)
		largestSeriesQuery := fmt.Sprintf(`
            SELECT series_name, SUM(estimated_gb) AS total_gb
            FROM (
                SELECT %s AS series_key,
                       %s AS series_name,
                       COALESCE(
                         CASE WHEN file_size_bytes IS NOT NULL AND file_size_bytes > 0
                              THEN file_size_bytes / 1073741824.0
                         END,
                         CASE WHEN bitrate_bps > 0 AND run_time_ticks > 0
                              THEN (bitrate_bps * (run_time_ticks / 10000000.0) / 8.0) / 1073741824.0
                         END,
                         (COALESCE(run_time_ticks, 0) / 36000000000.0) * 
                         CASE
                            WHEN COALESCE(height,0) >= 2160 THEN 25.0
                            WHEN COALESCE(height,0) >= 1080 THEN 8.0
                            WHEN COALESCE(height,0) >= 720  THEN 4.0
                            ELSE 2.0
                         END
                       ) AS estimated_gb
                FROM library_item
                WHERE %s
            )
            WHERE series_key IS NOT NULL AND series_key != '' AND series_name IS NOT NULL AND series_name != ''
            GROUP BY series_key, series_name
            ORDER BY total_gb DESC
            LIMIT 1
        `, seriesKeyExpr, seriesResolvedNameExpr, episodeWhere)
		err = db.QueryRow(largestSeriesQuery, episodeArgs...).Scan(&data.LargestSeriesName, &data.LargestSeriesGB)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[series] Error finding largest series: %v", err)
		}

		// Largest single episode by size: prefer file size, then bitrate*runtime, else heuristic
		largestEpisodeQuery := fmt.Sprintf(`
            SELECT name,
                   COALESCE(
                     CASE WHEN file_size_bytes IS NOT NULL AND file_size_bytes > 0
                          THEN file_size_bytes / 1073741824.0
                     END,
                     CASE WHEN bitrate_bps > 0 AND run_time_ticks > 0
                          THEN (bitrate_bps * (run_time_ticks / 10000000.0) / 8.0) / 1073741824.0
                     END,
                     (COALESCE(run_time_ticks, 0) / 36000000000.0) *
                     CASE
                         WHEN COALESCE(height,0) >= 2160 THEN 25.0
                         WHEN COALESCE(height,0) >= 1080 THEN 8.0
                         WHEN COALESCE(height,0) >= 720  THEN 4.0
                         ELSE 2.0
                     END
                   ) AS estimated_gb
            FROM library_item
            WHERE %s
            ORDER BY estimated_gb DESC
            LIMIT 1
        `, episodeWhere)
		err = db.QueryRow(largestEpisodeQuery, episodeArgs...).Scan(&data.LargestEpisodeName, &data.LargestEpisodeGB)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[series] Error finding largest episode: %v", err)
		}

		// Longest TV series by total runtime
		longestSeriesQuery := fmt.Sprintf(`
            SELECT series_name, SUM(run_time_ticks) / 600000000 AS minutes
            FROM (
                SELECT %s AS series_key, %s AS series_name, run_time_ticks
                FROM library_item
                WHERE %s AND run_time_ticks > 0
            )
            WHERE series_key IS NOT NULL AND series_key != '' AND series_name IS NOT NULL AND series_name != ''
            GROUP BY series_key, series_name
            ORDER BY minutes DESC
            LIMIT 1
        `, seriesKeyExpr, seriesResolvedNameExpr, episodeWhere)
		err = db.QueryRow(longestSeriesQuery, episodeArgs...).Scan(&data.LongestSeriesName, &data.LongestSeriesMinutes)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[series] Error finding longest series: %v", err)
		}

		// Most watched series (sum watch time across episodes of same series)
		mostWatchedQuery := fmt.Sprintf(`
            SELECT series_name, SUM(hours) AS total_hours FROM (
                SELECT %s AS series_key,
                       %s AS series_name,
                       SUM(pi.duration_seconds) / 3600.0 AS hours
                FROM play_intervals pi
                JOIN library_item li ON pi.item_id = li.id
                WHERE %s
                GROUP BY li.id
            )
            WHERE series_key IS NOT NULL AND series_key != '' AND series_name IS NOT NULL AND series_name != ''
            GROUP BY series_key, series_name
            ORDER BY total_hours DESC
            LIMIT 1
        `, seriesKeyExpr, seriesResolvedNameExpr, episodeAliasWhere)
		err = db.QueryRow(mostWatchedQuery, episodeAliasArgs...).Scan(&data.MostWatchedSeries.Name, &data.MostWatchedSeries.Hours)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[series] Error finding most watched series: %v", err)
		}

		// Total episode runtime hours (time to watch the whole TV library)
		totalEpisodeRuntimeQuery := fmt.Sprintf(`
            SELECT COALESCE(SUM(run_time_ticks), 0) / 36000000000.0
            FROM library_item
            WHERE %s AND run_time_ticks > 0
        `, episodeWhere)
		err = db.QueryRow(totalEpisodeRuntimeQuery, episodeArgs...).Scan(&data.TotalEpisodeRuntimeHours)
		if err != nil {
			log.Printf("[series] Error calculating total episode runtime: %v", err)
		}

		// Newest added episode -> return the episode display name and date
		newestSeriesQuery := fmt.Sprintf(`
            SELECT name, created_at
            FROM library_item
            WHERE %s
            ORDER BY created_at DESC
            LIMIT 1
        `, episodeWhere)
		err = db.QueryRow(newestSeriesQuery, episodeArgs...).Scan(&data.NewestSeries.Name, &data.NewestSeries.Date)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[series] Error finding newest series: %v", err)
		}

		// Episodes added this month
		addedEpisodesQuery := fmt.Sprintf(`
            SELECT COUNT(*)
            FROM library_item
            WHERE %s AND created_at >= date('now', 'start of month')
        `, episodeWhere)
		err = db.QueryRow(addedEpisodesQuery, episodeArgs...).Scan(&data.EpisodesAddedThisMonth)
		if err != nil {
			log.Printf("[series] Error counting episodes added this month: %v", err)
		}

		// Popular genres for series (based on episodes' any-genre tokens; top 5)
		{
			var cnt int
			row := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('library_item') WHERE name = 'genres'`)
			if err := row.Scan(&cnt); err == nil && cnt > 0 {
				genreQuery := fmt.Sprintf(`
                WITH RECURSIVE base AS (
                  SELECT id, series_id, REPLACE(genres, ', ', ',') AS g
                  FROM library_item
                  WHERE %s AND genres IS NOT NULL AND genres != '' AND series_id IS NOT NULL AND TRIM(series_id) != ''
                ),
                split(series_id, token, rest) AS (
                  SELECT series_id,
                         TRIM(CASE WHEN INSTR(g, ',') = 0 THEN g ELSE SUBSTR(g, 1, INSTR(g, ',') - 1) END),
                         TRIM(CASE WHEN INSTR(g, ',') = 0 THEN '' ELSE SUBSTR(g, INSTR(g, ',') + 1) END)
                  FROM base
                  UNION ALL
                  SELECT series_id,
                         TRIM(CASE WHEN INSTR(rest, ',') = 0 THEN rest ELSE SUBSTR(rest, 1, INSTR(rest, ',') - 1) END),
                         TRIM(CASE WHEN INSTR(rest, ',') = 0 THEN '' ELSE SUBSTR(rest, INSTR(rest, ',') + 1) END)
                  FROM split
                  WHERE rest <> ''
                )
                SELECT token AS genre, COUNT(DISTINCT series_id) AS count
                FROM split
                WHERE token IS NOT NULL AND token != ''
                GROUP BY token
                ORDER BY count DESC
                LIMIT 5`, episodeWhere)
				genreArgs := append([]interface{}{}, episodeArgs...)
				if rows, err := db.Query(genreQuery, genreArgs...); err == nil {
					defer rows.Close()
					for rows.Next() {
						var g GenreStats
						if err := rows.Scan(&g.Genre, &g.Count); err == nil {
							data.PopularGenres = append(data.PopularGenres, g)
						}
					}
				} else {
					log.Printf("[series] Error fetching popular genres: %v", err)
				}
			} else {
				log.Printf("[series] 'genres' column not found; skipping popular genres")
			}
		}

		duration := time.Since(start)
		isSlowQuery := duration > 1*time.Second
		if isSlowQuery {
			log.Printf("[series] WARNING: Slow query took %v", duration)
		}
		admin.IncrementQueryMetrics(duration, isSlowQuery)

		log.Printf("[series] OK in %v: series=%d episodes=%d longest=%s most_watched=%s",
			duration, data.TotalSeries, data.TotalEpisodes, data.LongestSeriesName, data.MostWatchedSeries.Name)

		return c.JSON(data)
	}
}
