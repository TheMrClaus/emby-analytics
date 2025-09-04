package stats

import (
    "database/sql"
    "log"
    "time"

    "emby-analytics/internal/handlers/admin"

    "github.com/gofiber/fiber/v3"
)

// SeriesData holds aggregated stats across TV series/episodes.
type SeriesData struct {
    TotalSeries   int `json:"total_series"`
    TotalEpisodes int `json:"total_episodes"}`

    LargestSeriesName string  `json:"largest_series_name"`
    LargestSeriesGB   float64 `json:"largest_series_total_gb"`

    LargestEpisodeName string  `json:"largest_episode_name"`
    LargestEpisodeGB   float64 `json:"largest_episode_gb"`

    LongestSeriesName   string `json:"longest_series_name"`
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

    EpisodesAddedThisMonth int `json:"episodes_added_this_month"`
}

// helper SQL fragment to derive a series name from the episode's display name convention
// We assume episodes have name like "Series Name - Episode Name (SxxExx)" after enrichment,
// otherwise this returns NULL/empty.
const seriesNameExpr = "TRIM(CASE WHEN INSTR(name, ' - ') > 0 THEN SUBSTR(name, 1, INSTR(name, ' - ') - 1) ELSE NULL END)"

func Series(db *sql.DB) fiber.Handler {
    return func(c fiber.Ctx) error {
        start := time.Now()
        data := SeriesData{}

        // Total series: count distinct derived series names among episodes
        err := db.QueryRow(`
            SELECT COUNT(*) FROM (
                SELECT DISTINCT `+seriesNameExpr+` AS series
                FROM library_item
                WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+`
            ) WHERE series IS NOT NULL AND series != ''
        `).Scan(&data.TotalSeries)
        if err != nil {
            log.Printf("[series] Error counting series: %v", err)
            return c.Status(500).JSON(fiber.Map{"error": "Failed to count series"})
        }

        // Total episodes
        err = db.QueryRow(`
            SELECT COUNT(*)
            FROM library_item
            WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+`
        `).Scan(&data.TotalEpisodes)
        if err != nil {
            log.Printf("[series] Error counting episodes: %v", err)
        }

        // Largest TV series by total size across episodes (prefer file size, then bitrate*runtime, else heuristic)
        err = db.QueryRow(`
            SELECT series, SUM(estimated_gb) AS total_gb
            FROM (
                SELECT `+seriesNameExpr+` AS series,
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
                WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+`
            )
            WHERE series IS NOT NULL AND series != ''
            GROUP BY series
            ORDER BY total_gb DESC
            LIMIT 1
        `).Scan(&data.LargestSeriesName, &data.LargestSeriesGB)
        if err != nil && err != sql.ErrNoRows {
            log.Printf("[series] Error finding largest series: %v", err)
        }

        // Largest single episode by size: prefer file size, then bitrate*runtime, else heuristic
        err = db.QueryRow(`
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
            WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+`
            ORDER BY estimated_gb DESC
            LIMIT 1
        `).Scan(&data.LargestEpisodeName, &data.LargestEpisodeGB)
        if err != nil && err != sql.ErrNoRows {
            log.Printf("[series] Error finding largest episode: %v", err)
        }

        // Longest TV series by total runtime
        err = db.QueryRow(`
            SELECT series, SUM(run_time_ticks) / 600000000 AS minutes
            FROM (
                SELECT `+seriesNameExpr+` AS series, run_time_ticks
                FROM library_item
                WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+` AND run_time_ticks > 0
            )
            WHERE series IS NOT NULL AND series != ''
            GROUP BY series
            ORDER BY minutes DESC
            LIMIT 1
        `).Scan(&data.LongestSeriesName, &data.LongestSeriesMinutes)
        if err != nil && err != sql.ErrNoRows {
            log.Printf("[series] Error finding longest series: %v", err)
        }

        // Most watched series (sum watch time across episodes of same series)
        err = db.QueryRow(`
            SELECT series, SUM(hours) AS total_hours FROM (
                SELECT `+seriesNameExpr+` AS series, SUM(pi.duration_seconds) / 3600.0 AS hours
                FROM play_intervals pi
                JOIN library_item li ON pi.item_id = li.id
                WHERE li.media_type = 'Episode' AND `+excludeLiveTvFilter()+`
                GROUP BY li.id
            )
            WHERE series IS NOT NULL AND series != ''
            GROUP BY series
            ORDER BY total_hours DESC
            LIMIT 1
        `).Scan(&data.MostWatchedSeries.Name, &data.MostWatchedSeries.Hours)
        if err != nil && err != sql.ErrNoRows {
            log.Printf("[series] Error finding most watched series: %v", err)
        }

        // Total episode runtime hours (time to watch the whole TV library)
        err = db.QueryRow(`
            SELECT COALESCE(SUM(run_time_ticks), 0) / 36000000000.0
            FROM library_item
            WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+` AND run_time_ticks > 0
        `).Scan(&data.TotalEpisodeRuntimeHours)
        if err != nil {
            log.Printf("[series] Error calculating total episode runtime: %v", err)
        }

        // Newest added episode -> return the episode display name and date
        err = db.QueryRow(`
            SELECT name, created_at
            FROM library_item
            WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+`
            ORDER BY created_at DESC
            LIMIT 1
        `).Scan(&data.NewestSeries.Name, &data.NewestSeries.Date)
        if err != nil && err != sql.ErrNoRows {
            log.Printf("[series] Error finding newest series: %v", err)
        }

        // Episodes added this month
        err = db.QueryRow(`
            SELECT COUNT(*)
            FROM library_item
            WHERE media_type = 'Episode' AND `+excludeLiveTvFilter()+` AND created_at >= date('now', 'start of month')
        `).Scan(&data.EpisodesAddedThisMonth)
        if err != nil {
            log.Printf("[series] Error counting episodes added this month: %v", err)
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
