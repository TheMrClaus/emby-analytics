package sync

import (
	"context"
	"database/sql"
	"log"
	"time"

	"emby-analytics/internal/emby"
	"emby-analytics/internal/handlers/admin"
)

// Scheduler manages automatic sync operations
type Scheduler struct {
	db  *sql.DB
	em  *emby.Client
	rm  *admin.RefreshManager
	ctx context.Context
	cancel context.CancelFunc
}

// NewScheduler creates a new sync scheduler
func NewScheduler(db *sql.DB, em *emby.Client, rm *admin.RefreshManager) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		db:     db,
		em:     em,
		rm:     rm,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the automatic sync scheduling
func (s *Scheduler) Start() {
	log.Println("[scheduler] 🕐 Starting smart sync scheduler")
	
	// Start incremental sync ticker (every 5 minutes)
	incrementalTicker := time.NewTicker(5 * time.Minute)
	
	// Start daily full sync ticker (check every hour for 3 AM)
	dailyTicker := time.NewTicker(1 * time.Hour)
	
	go func() {
		defer incrementalTicker.Stop()
		defer dailyTicker.Stop()
		
		// Run initial incremental sync after 30 seconds
		initialTimer := time.NewTimer(30 * time.Second)
		
		for {
			select {
			case <-s.ctx.Done():
				log.Println("[scheduler] 🛑 Sync scheduler stopped")
				return
				
			case <-initialTimer.C:
				log.Println("[scheduler] 🔄 Running initial incremental sync")
				s.runIncrementalSync()
				
			case <-incrementalTicker.C:
				log.Println("[scheduler] 🔄 Running scheduled incremental sync")
				s.runIncrementalSync()
				
			case <-dailyTicker.C:
				if s.shouldRunDailySync() {
					log.Println("[scheduler] 🌙 Running nightly full sync")
					s.runFullSync()
				}
			}
		}
	}()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// runIncrementalSync performs an incremental sync if conditions are met
func (s *Scheduler) runIncrementalSync() {
	// Check if refresh manager is already running
	status := s.rm.get()
	if status.Running {
		log.Println("[scheduler] ⏸️  Skipping incremental sync - refresh already running")
		return
	}
	
	// Check if last incremental sync was recent (avoid too frequent syncs)
	lastSync, err := GetLastSyncTime(s.db, SyncTypeLibraryIncremental)
	if err == nil && time.Since(*lastSync) < 2*time.Minute {
		log.Println("[scheduler] ⏸️  Skipping incremental sync - too recent")
		return
	}
	
	log.Println("[scheduler] ⚡ Starting incremental sync")
	s.rm.StartIncremental(s.db, s.em)
}

// runFullSync performs a full library sync
func (s *Scheduler) runFullSync() {
	// Check if refresh manager is already running
	status := s.rm.get()
	if status.Running {
		log.Println("[scheduler] ⏸️  Skipping full sync - refresh already running")
		return
	}
	
	log.Println("[scheduler] 🌟 Starting nightly full sync")
	s.rm.Start(s.db, s.em, 200) // Use smaller chunk size for nightly sync
}

// shouldRunDailySync checks if it's time for the daily full sync (around 3 AM)
func (s *Scheduler) shouldRunDailySync() bool {
	now := time.Now()
	
	// Check if it's between 3:00 AM and 3:59 AM
	if now.Hour() != 3 {
		return false
	}
	
	// Check if we already ran a full sync today
	lastSync, err := GetLastSyncTime(s.db, SyncTypeLibraryFull)
	if err != nil {
		// If we can't get last sync time, assume we should sync
		return true
	}
	
	// If last full sync was today, don't run again
	today := time.Now().Format("2006-01-02")
	lastSyncDay := lastSync.Format("2006-01-02")
	
	return today != lastSyncDay
}

// GetSchedulerStats returns statistics about the scheduler
func GetSchedulerStats(db *sql.DB) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Get last incremental sync
	incrementalSync, err := GetSyncStats(db, SyncTypeLibraryIncremental)
	if err == nil {
		stats["last_incremental_sync"] = incrementalSync.Format("2006-01-02 15:04:05")
		stats["incremental_sync_age_minutes"] = int(time.Since(incrementalSync).Minutes())
	}
	
	// Get last full sync
	fullSync, err := GetSyncStats(db, SyncTypeLibraryFull)
	if err == nil {
		stats["last_full_sync"] = fullSync.Format("2006-01-02 15:04:05")
		stats["full_sync_age_hours"] = int(time.Since(fullSync).Hours())
	}
	
	// Add scheduling info
	stats["incremental_sync_interval"] = "5 minutes"
	stats["full_sync_schedule"] = "3:00 AM daily"
	stats["next_incremental_sync"] = "within 5 minutes"
	
	// Calculate next full sync time
	now := time.Now()
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if now.After(next3AM) {
		next3AM = next3AM.Add(24 * time.Hour)
	}
	stats["next_full_sync"] = next3AM.Format("2006-01-02 15:04:05")
	
	return stats, nil
}