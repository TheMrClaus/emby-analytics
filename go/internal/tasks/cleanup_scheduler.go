package tasks

import (
	"context"
	"database/sql"
	"time"

	"emby-analytics/internal/audit"
	"emby-analytics/internal/cleanup"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/logging"
)

// CleanupScheduler manages automatic cleanup operations
type CleanupScheduler struct {
	db     *sql.DB
	em     *emby.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// NewCleanupScheduler creates a new cleanup scheduler
func NewCleanupScheduler(db *sql.DB, em *emby.Client) *CleanupScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &CleanupScheduler{
		db:     db,
		em:     em,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the automatic cleanup scheduling
func (s *CleanupScheduler) Start() {
	logging.Info("Starting cleanup scheduler")

	// Start weekly cleanup ticker (check every 6 hours for Sunday 2 AM)
	weeklyTicker := time.NewTicker(6 * time.Hour)

	go func() {
		defer weeklyTicker.Stop()

		// Run initial cleanup check after 5 minutes (let system stabilize)
		initialTimer := time.NewTimer(5 * time.Minute)

		for {
			select {
			case <-s.ctx.Done():
				logging.Info("Cleanup scheduler stopped")
				return

			case <-initialTimer.C:
				logging.Debug("Running initial cleanup check")
				if s.shouldRunWeeklyCleanup() {
					s.runWeeklyCleanup()
				}

			case <-weeklyTicker.C:
				if s.shouldRunWeeklyCleanup() {
					logging.Info("Running scheduled weekly cleanup")
					s.runWeeklyCleanup()
				}
			}
		}
	}()
}

// Stop stops the cleanup scheduler
func (s *CleanupScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// runWeeklyCleanup performs automatic cleanup of stale library items
func (s *CleanupScheduler) runWeeklyCleanup() {
	if s.em == nil {
		logging.Warn("Cleanup scheduler: Emby client not configured, skipping cleanup")
		return
	}

	logger, err := audit.NewCleanupLogger(s.db, "scheduled-cleanup", "scheduler")
	if err != nil {
		logging.Error("Cleanup scheduler: Failed to initialize audit log", "error", err)
		return
	}

	logging.Info("Starting scheduled cleanup", "job_id", logger.GetJobID())
	
	// Use higher limit for scheduled cleanup (process more items)
	limit := 10000
	
	// Get library items to check
	rows, err := s.db.Query(`
		SELECT id, name, media_type, series_name 
		FROM library_item 
		LIMIT ?
	`, limit)
	if err != nil {
		logger.FailJob(err.Error())
		logging.Error("Cleanup scheduler: Failed to query library items", "error", err)
		return
	}
	defer rows.Close()

	items := []itemInfo{}
	ids := []string{}
	for rows.Next() {
		var item itemInfo
		var seriesName sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.MediaType, &seriesName); err == nil {
			if seriesName.Valid {
				item.SeriesName = seriesName.String
			}
			items = append(items, item)
			ids = append(ids, item.ID)
		}
	}

	if len(ids) == 0 {
		summary := map[string]interface{}{"result": "no_items"}
		logger.CompleteJob(0, 0, summary)
		logging.Debug("Cleanup scheduler: No items to process")
		return
	}

	// Check existence in Emby
	chunk := 50
	found := make(map[string]struct{}, len(ids))
	for i := 0; i < len(ids); i += chunk {
		end := i + chunk
		if end > len(ids) {
			end = len(ids)
		}
		part := ids[i:end]
		embyItems, err := s.em.ItemsByIDs(part)
		if err != nil {
			logger.FailJob(err.Error())
			logging.Error("Cleanup scheduler: Failed to check Emby items", "error", err)
			return
		}
		for _, it := range embyItems {
			found[it.Id] = struct{}{}
		}
	}

	// Process missing items
	deleted, merged, skipped := 0, 0, 0
	missingItems := []itemInfo{}

	for _, item := range items {
		if _, ok := found[item.ID]; !ok {
			missingItems = append(missingItems, item)
		}
	}

	// Process each missing item
	for _, item := range missingItems {
		var hasIntervals int
		_ = s.db.QueryRow(`SELECT 1 FROM play_intervals WHERE item_id = ? LIMIT 1`, item.ID).Scan(&hasIntervals)
		
		if hasIntervals == 0 {
			// Safe to delete - no watch history
			if _, err := s.db.Exec(`DELETE FROM library_item WHERE id = ?`, item.ID); err == nil {
				deleted++
				logger.LogItemAction("deleted", item.ID, item.Name, item.MediaType, "", 
					map[string]interface{}{"reason": "no_watch_history"})
			}
		} else {
			// Try to merge with existing item
			targetID, err := cleanup.FindMatchingItem(s.db, cleanup.ItemInfo(item))
			if err != nil || targetID == "" {
				skipped++
				logger.LogItemAction("skipped", item.ID, item.Name, item.MediaType, "", 
					map[string]interface{}{"reason": "no_matching_target"})
				continue
			}
			
			// Attempt merge
			if err := cleanup.MergeItemData(s.db, item.ID, targetID); err != nil {
				skipped++
				logger.LogItemAction("skipped", item.ID, item.Name, item.MediaType, targetID,
					map[string]interface{}{"reason": "merge_failed", "error": err.Error()})
			} else {
				merged++
				logger.LogItemAction("merged", item.ID, item.Name, item.MediaType, targetID,
					map[string]interface{}{"reason": "duplicate_found"})
			}
		}
	}

	// Complete audit log
	summary := map[string]interface{}{
		"deleted":       deleted,
		"merged":        merged,
		"skipped":       skipped,
		"total_missing": len(missingItems),
		"total_checked": len(ids),
	}
	logger.CompleteJob(len(ids), deleted+merged, summary)

	logging.Info("Scheduled cleanup completed", 
		"job_id", logger.GetJobID(),
		"checked", len(ids),
		"missing", len(missingItems), 
		"deleted", deleted,
		"merged", merged,
		"skipped", skipped)
}

// shouldRunWeeklyCleanup checks if it's time for weekly cleanup (Sunday 2 AM)
func (s *CleanupScheduler) shouldRunWeeklyCleanup() bool {
	now := time.Now()

	// Check if it's Sunday (0) between 2:00 AM and 2:59 AM  
	if now.Weekday() != time.Sunday || now.Hour() != 2 {
		return false
	}

	// Check if we already ran cleanup this week
	var lastRunUnix sql.NullInt64
	err := s.db.QueryRow(`
		SELECT MAX(started_at) 
		FROM cleanup_jobs 
		WHERE operation_type = 'scheduled-cleanup' 
		AND created_by = 'scheduler'
		AND started_at > ?
	`, time.Now().AddDate(0, 0, -7).Unix()).Scan(&lastRunUnix)
	
	if err != nil {
		// If we can't check, assume we should run
		return true
	}
	
	if !lastRunUnix.Valid {
		// No recent runs, should run
		return true
	}

	// If last run was within the last 6 days, don't run again
	lastTime := time.Unix(lastRunUnix.Int64, 0)
	return time.Since(lastTime) >= 6*24*time.Hour
}

// itemInfo represents a library item with metadata
type itemInfo struct {
	ID         string
	Name       string
	MediaType  string
	SeriesName string
}



// GetCleanupStats returns statistics about scheduled cleanup operations  
func GetCleanupStats(db *sql.DB) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Get last scheduled cleanup
	var lastRunUnix sql.NullInt64
	var itemsProcessed sql.NullInt64
	err := db.QueryRow(`
		SELECT started_at, items_processed
		FROM cleanup_jobs 
		WHERE operation_type = 'scheduled-cleanup' 
		AND created_by = 'scheduler'
		AND status = 'completed'
		ORDER BY started_at DESC 
		LIMIT 1
	`).Scan(&lastRunUnix, &itemsProcessed)
	
	if err == nil && lastRunUnix.Valid {
		lastTime := time.Unix(lastRunUnix.Int64, 0)
		stats["last_cleanup"] = lastTime.Format("2006-01-02 15:04:05")
		stats["cleanup_age_hours"] = int(time.Since(lastTime).Hours())
		if itemsProcessed.Valid {
			stats["items_processed"] = itemsProcessed.Int64
		}
	}
	
	// Add scheduling info
	stats["cleanup_schedule"] = "Sunday 2:00 AM weekly"
	
	// Calculate next cleanup time
	now := time.Now()
	nextSunday := now
	for nextSunday.Weekday() != time.Sunday {
		nextSunday = nextSunday.Add(24 * time.Hour)
	}
	next2AM := time.Date(nextSunday.Year(), nextSunday.Month(), nextSunday.Day(), 2, 0, 0, 0, now.Location())
	if now.After(next2AM) {
		next2AM = next2AM.Add(7 * 24 * time.Hour)
	}
	stats["next_cleanup"] = next2AM.Format("2006-01-02 15:04:05")
	
	return stats, nil
}