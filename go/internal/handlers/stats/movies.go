package stats

import (
	"database/sql"
	"log"
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
	TotalRuntimeHours   float64           `json:"total_runtime_hours"`
	PopularGenres       []GenreStats      `json:"popular_genres"`
	MoviesAddedThisMonth int              `json:"movies_added_this_month"`
}

type GenreStats struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

func Movies(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		data := MoviesData{}

		// Count total movies
		err := db.QueryRow(`
			SELECT COUNT(*) 
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()).Scan(&data.TotalMovies)
		if err != nil {
			log.Printf("[movies] Error counting movies: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to count movies"})
		}

        // Get largest movie: prefer actual size, then bitrate*runtime, else heuristic
        err = db.QueryRow(`
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
                       WHEN height >= 2160 THEN 25.0  -- 4K estimate
                       WHEN height >= 1080 THEN 8.0   -- 1080p estimate
                       WHEN height >= 720 THEN 4.0    -- 720p estimate
                       ELSE 2.0                        -- SD estimate
                     END
                   ) AS estimated_gb
            FROM library_item
            WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+`
            ORDER BY estimated_gb DESC
            LIMIT 1`).Scan(&data.LargestMovieName, &data.LargestMovieGB)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding largest movie: %v", err)
		}

		// Get longest runtime movie
		err = db.QueryRow(`
			SELECT name, run_time_ticks / 600000000 
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+` 
			  AND run_time_ticks > 0
			ORDER BY run_time_ticks DESC 
			LIMIT 1`).Scan(&data.LongestMovieName, &data.LongestRuntime)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding longest movie: %v", err)
		}

		// Get shortest runtime movie (but reasonable minimum of 30 minutes)
		err = db.QueryRow(`
			SELECT name, run_time_ticks / 600000000 
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+` 
			  AND run_time_ticks >= 18000000000  -- At least 30 minutes
			ORDER BY run_time_ticks ASC 
			LIMIT 1`).Scan(&data.ShortestMovieName, &data.ShortestRuntime)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding shortest movie: %v", err)
		}

		// Get newest added movie
		err = db.QueryRow(`
			SELECT name, created_at 
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+`
			ORDER BY created_at DESC 
			LIMIT 1`).Scan(&data.NewestMovie.Name, &data.NewestMovie.Date)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding newest movie: %v", err)
		}

		// Get most watched movie from play_intervals
		err = db.QueryRow(`
			SELECT li.name, SUM(pi.duration_seconds) / 3600.0 as hours
			FROM play_intervals pi
			JOIN library_item li ON pi.item_id = li.id
			WHERE li.media_type = 'Movie' AND `+excludeLiveTvFilter()+`
			GROUP BY pi.item_id, li.name
			ORDER BY hours DESC
			LIMIT 1`).Scan(&data.MostWatchedMovie.Name, &data.MostWatchedMovie.Hours)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[movies] Error finding most watched movie: %v", err)
		}

		// Calculate total runtime hours
		err = db.QueryRow(`
			SELECT COALESCE(SUM(run_time_ticks), 0) / 36000000000.0 
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+` 
			  AND run_time_ticks > 0`).Scan(&data.TotalRuntimeHours)
		if err != nil {
			log.Printf("[movies] Error calculating total runtime: %v", err)
		}

		// Get popular genres (top 5) - simplified approach
		// Note: This assumes genres are stored as comma-separated values
		genreRows, err := db.Query(`
			SELECT 
				CASE 
					WHEN INSTR(genres, ',') > 0 THEN TRIM(SUBSTR(genres, 1, INSTR(genres, ',') - 1))
					ELSE TRIM(genres)
				END as primary_genre,
				COUNT(*) as count
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+` 
			  AND genres IS NOT NULL AND genres != ''
			GROUP BY primary_genre
			HAVING primary_genre != ''
			ORDER BY count DESC
			LIMIT 5`)
		
		if err == nil {
			defer genreRows.Close()
			for genreRows.Next() {
				var genre GenreStats
				if err := genreRows.Scan(&genre.Genre, &genre.Count); err == nil {
					data.PopularGenres = append(data.PopularGenres, genre)
				}
			}
		} else {
			log.Printf("[movies] Error fetching popular genres: %v", err)
		}

		// Get movies added this month
		err = db.QueryRow(`
			SELECT COUNT(*) 
			FROM library_item 
			WHERE media_type = 'Movie' AND `+excludeLiveTvFilter()+` 
			  AND created_at >= date('now', 'start of month')`).Scan(&data.MoviesAddedThisMonth)
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
