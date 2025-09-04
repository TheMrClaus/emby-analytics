package sync

import (
    "context"
    "database/sql"
    "time"

    "emby-analytics/internal/emby"
    "emby-analytics/internal/logging"
)

// Scheduler manages automatic sync operations
type Scheduler struct {
	db  *sql.DB
	em  *emby.Client
	rm  RefreshManager
	ctx context.Context
	cancel context.CancelFunc
}

// NewScheduler creates a new sync scheduler
func NewScheduler(db *sql.DB, em *emby.Client, rm RefreshManager) *Scheduler {
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
    logging.Info("Starting smart sync scheduler")
    
    // Start incremental sync ticker (every 5 minutes)
    incrementalTicker := time.NewTicker(5 * time.Minute)
    
    // Start daily full sync ticker (check every hour for 3 AM)
    dailyTicker := time.NewTicker(1 * time.Hour)

    // Start active session ingest ticker (every 1 minute)
    ingestTicker := time.NewTicker(1 * time.Minute)
	
	go func() {
        defer incrementalTicker.Stop()
        defer dailyTicker.Stop()
        defer ingestTicker.Stop()
		
		// Run initial incremental sync after 30 seconds
		initialTimer := time.NewTimer(30 * time.Second)
		
		for {
			select {
			case <-s.ctx.Done():
				logging.Info("Sync scheduler stopped")
				return
				
			case <-initialTimer.C:
				logging.Info("Running initial incremental sync")
				s.runIncrementalSync()
				
            case <-incrementalTicker.C:
                logging.Info("Running scheduled incremental sync")
                s.runIncrementalSync()
                
            case <-dailyTicker.C:
                if s.shouldRunDailySync() {
                    logging.Info("Running nightly full sync")
                    s.runFullSync()
                }

            case <-ingestTicker.C:
                s.runActiveSessionIngest()
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
	status := s.rm.Get()
	if status.Running {
		logging.Debug("Skipping incremental sync - refresh already running")
		return
	}
	
	// Check if last incremental sync was recent (avoid too frequent syncs)
	lastSync, err := GetLastSyncTime(s.db, SyncTypeLibraryIncremental)
	if err == nil && time.Since(*lastSync) < 2*time.Minute {
		logging.Debug("Skipping incremental sync - too recent")
		return
	}
	
	logging.Info("Starting incremental sync")
	s.rm.StartIncremental(s.db, s.em)
}

// runFullSync performs a full library sync
func (s *Scheduler) runFullSync() {
	// Check if refresh manager is already running
	status := s.rm.Get()
	if status.Running {
		logging.Debug("Skipping full sync - refresh already running")
		return
	}
	
	logging.Info("Starting nightly full sync")
	s.rm.Start(s.db, s.em, 200) // Use smaller chunk size for nightly sync
}

// runActiveSessionIngest ensures a play_sessions row exists for each active Emby session.
func (s *Scheduler) runActiveSessionIngest() {
    sessions, err := s.em.GetActiveSessions()
    if err != nil {
        logging.Warn("Active ingest failed to fetch sessions", "error", err)
        return
    }
    if len(sessions) == 0 {
        return
    }
    now := time.Now().UTC().Unix()
    inserted, updated := 0, 0
    for _, es := range sessions {
        var id int64
        selErr := s.db.QueryRow(`SELECT id FROM play_sessions WHERE session_id=? AND item_id=?`, es.SessionID, es.ItemID).Scan(&id)
        if selErr == nil {
            // Update existing row to active and refresh details
            _, _ = s.db.Exec(`
                UPDATE play_sessions 
                SET user_id=?, device_id=?, client_name=?, item_name=?, item_type=?, play_method=?,
                    started_at=?, ended_at=NULL, is_active=true, transcode_reasons=?, remote_address=?,
                    video_method=?, audio_method=?, video_codec_from=?, video_codec_to=?,
                    audio_codec_from=?, audio_codec_to=?
                WHERE id=?
            `, es.UserID, es.Device, es.App, es.ItemName, es.ItemType, es.PlayMethod, now,
               joinReasons(es.TransReasons), es.RemoteAddress,
               es.VideoMethod, es.AudioMethod, es.TransVideoFrom, es.TransVideoTo, es.TransAudioFrom, es.TransAudioTo, id)
            updated++
            continue
        }
        // Insert missing row
        _, _ = s.db.Exec(`
            INSERT INTO play_sessions
            (user_id, session_id, device_id, client_name, item_id, item_name, item_type, play_method, started_at, is_active, transcode_reasons, remote_address, video_method, audio_method, video_codec_from, video_codec_to, audio_codec_from, audio_codec_to)
            VALUES(?,?,?,?,?,?,?,?,?,true,?,?,?,?,?,?,?)
        `, es.UserID, es.SessionID, es.Device, es.App, es.ItemID, es.ItemName, es.ItemType, es.PlayMethod, now,
           joinReasons(es.TransReasons), es.RemoteAddress, es.VideoMethod, es.AudioMethod, es.TransVideoFrom, es.TransVideoTo, es.TransAudioFrom, es.TransAudioTo)
        inserted++
    }
    if inserted+updated > 0 {
        logging.Debug("Active ingest completed", "upserted", inserted+updated, "inserted", inserted, "updated", updated)
    }
}

func joinReasons(rs []string) string {
    if len(rs) == 0 {
        return ""
    }
    out := rs[0]
    for i := 1; i < len(rs); i++ {
        out += "," + rs[i]
    }
    return out
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
	incrementalSync, incrementalItems, err := GetSyncStats(db, SyncTypeLibraryIncremental)
	if err == nil {
		stats["last_incremental_sync"] = incrementalSync.Format("2006-01-02 15:04:05")
		stats["incremental_sync_age_minutes"] = int(time.Since(incrementalSync).Minutes())
		stats["incremental_items_processed"] = incrementalItems
	}
	
	// Get last full sync
	fullSync, fullItems, err := GetSyncStats(db, SyncTypeLibraryFull)
	if err == nil {
		stats["last_full_sync"] = fullSync.Format("2006-01-02 15:04:05")
		stats["full_sync_age_hours"] = int(time.Since(fullSync).Hours())
		stats["full_items_processed"] = fullItems
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
