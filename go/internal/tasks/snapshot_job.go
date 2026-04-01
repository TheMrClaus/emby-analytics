package tasks

import (
	"database/sql"
	"time"

	"emby-analytics/internal/logging"
)

// StartSnapshotLoop runs a daily background job to capture library snapshots for storage analytics.
// Snapshots are taken at midnight local time.
func StartSnapshotLoop(db *sql.DB) {
	logging.Debug("Starting library snapshot loop (daily at midnight)")

	// Calculate time until next midnight
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	durationUntilMidnight := time.Until(nextMidnight)

	// Initial snapshot on startup
	go func() {
		time.Sleep(5 * time.Second) // Brief delay to let initial syncs complete
		if err := captureLibrarySnapshot(db); err != nil {
			logging.Debug("initial snapshot failed", "error", err)
		}
	}()

	// Schedule daily snapshots
	go func() {
		// Wait until first midnight
		time.Sleep(durationUntilMidnight)
		
		// Take snapshot at midnight
		if err := captureLibrarySnapshot(db); err != nil {
			logging.Debug("snapshot failed", "error", err)
		}

		// Continue with 24-hour ticker
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			<-ticker.C
			if err := captureLibrarySnapshot(db); err != nil {
				logging.Debug("snapshot failed", "error", err)
			}
		}
	}()
}

// captureLibrarySnapshot captures current library statistics into library_snapshots table.
func captureLibrarySnapshot(db *sql.DB) error {
	start := time.Now()
	logging.Debug("capturing library snapshot")

	// Get total counts and size
	var totalItems, totalSize int64
	var movieCount, seriesCount, episodeCount int64
	var video4k, video1080p, video720p, videoSD int64

	// Total items and size
	err := db.QueryRow(`
		SELECT 
			COUNT(*), 
			COALESCE(SUM(file_size_bytes), 0)
		FROM library_item
	`).Scan(&totalItems, &totalSize)
	if err != nil {
		return err
	}

	// Counts by type
	rows, err := db.Query(`
		SELECT 
			media_type,
			COUNT(*)
		FROM library_item
		WHERE media_type IN ('Movie', 'Series', 'Episode')
		GROUP BY media_type
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var itemType string
		var count int64
		if err := rows.Scan(&itemType, &count); err != nil {
			continue
		}
		switch itemType {
		case "Movie":
			movieCount = count
		case "Series":
			seriesCount = count
		case "Episode":
			episodeCount = count
		}
	}

	// Counts by video resolution
	qualityRows, err := db.Query(`
		SELECT 
			CASE
				WHEN height >= 2160 THEN '4K'
				WHEN height >= 1080 THEN '1080p'
				WHEN height >= 720 THEN '720p'
				ELSE 'SD'
			END as quality,
			COUNT(*)
		FROM library_item
		WHERE height IS NOT NULL
		GROUP BY quality
	`)
	if err != nil {
		return err
	}
	defer qualityRows.Close()

	for qualityRows.Next() {
		var quality string
		var count int64
		if err := qualityRows.Scan(&quality, &count); err != nil {
			continue
		}
		switch quality {
		case "4K":
			video4k = count
		case "1080p":
			video1080p = count
		case "720p":
			video720p = count
		case "SD":
			videoSD = count
		}
	}

	// Insert snapshot
	_, err = db.Exec(`
		INSERT INTO library_snapshots (
			snapshot_date,
			total_items,
			total_size_bytes,
			movie_count,
			series_count,
			episode_count,
			video_4k_count,
			video_1080p_count,
			video_720p_count,
			video_sd_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 
		time.Now().Format("2006-01-02"),
		totalItems,
		totalSize,
		movieCount,
		seriesCount,
		episodeCount,
		video4k,
		video1080p,
		video720p,
		videoSD,
	)
	if err != nil {
		return err
	}

	logging.Debug("library snapshot captured",
		"duration", time.Since(start).Round(time.Millisecond),
		"items", totalItems,
		"size_gb", float64(totalSize)/1073741824.0,
		"movies", movieCount,
		"series", seriesCount,
		"episodes", episodeCount,
	)

	return nil
}
