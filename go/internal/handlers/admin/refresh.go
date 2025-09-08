package admin

import (
	"emby-analytics/internal/logging"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
	syncpkg "emby-analytics/internal/sync"
	"emby-analytics/internal/types"
)

type Progress = types.Progress

type RefreshManager struct {
	mu       sync.Mutex
	progress Progress
}

func NewRefreshManager() *RefreshManager {
	return &RefreshManager{}
}

func (rm *RefreshManager) set(p Progress) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.progress = p
}

func (rm *RefreshManager) Get() Progress {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.progress
}

// get is kept for backward compatibility with existing handlers
func (rm *RefreshManager) get() Progress {
	return rm.Get()
}

// Start a background refresh with full sync
func (rm *RefreshManager) Start(db *sql.DB, em *emby.Client, chunkSize int) {
	rm.set(Progress{Message: "Starting full refresh...", Running: true})
	go rm.refreshWorker(db, em, chunkSize, false)
}

// StartIncremental starts a background incremental sync
func (rm *RefreshManager) StartIncremental(db *sql.DB, em *emby.Client) {
	rm.set(Progress{Message: "Starting incremental sync...", Running: true})
	go rm.refreshWorker(db, em, 1000, true)
}

func (rm *RefreshManager) refreshWorker(db *sql.DB, em *emby.Client, chunkSize int, incremental bool) {
	var total int
	var actualItemsProcessed int

	if incremental {
		// Phase 1: Incremental Library Metadata Refresh
		rm.set(Progress{Message: "Starting incremental sync...", Running: true})

		// Get last sync timestamp
		lastSync, err := syncpkg.GetLastSyncTime(db, syncpkg.SyncTypeLibraryIncremental)
		if err != nil {
			rm.set(Progress{Error: "Failed to get last sync time: " + err.Error(), Done: true})
			return
		}

		rm.set(Progress{Message: fmt.Sprintf("Fetching items modified since %s...", lastSync.Format("2006-01-02 15:04:05")), Running: true})

		// Fetch incremental items
		libraryEntries, totalFound, err := em.GetItemsIncremental(chunkSize, lastSync)
		if err != nil {
			rm.set(Progress{Error: "Failed to fetch incremental items: " + err.Error(), Done: true})
			return
		}

		total = totalFound
		actualItemsProcessed = len(libraryEntries)
		
		rm.set(Progress{
			Total: total,
			Processed: 0,
			Message: fmt.Sprintf("Processing %d new/modified items...", len(libraryEntries)),
			Running: true,
		})

		// Process the incremental items
		if len(libraryEntries) > 0 {
			dbEntriesInserted := rm.processLibraryEntries(db, em, libraryEntries)
			logging.Debug("Processed %d items, inserted/updated %d entries", len(libraryEntries), dbEntriesInserted)
		}

		// Update sync timestamp
		if err := syncpkg.UpdateSyncTime(db, syncpkg.SyncTypeLibraryIncremental, actualItemsProcessed); err != nil {
			logging.Debug("Failed to update sync timestamp: %v", err)
		}

		rm.set(Progress{
			Total: total,
			Processed: actualItemsProcessed,
			Message: fmt.Sprintf("Incremental sync complete! Processed %d items", actualItemsProcessed),
			Done: true,
			Running: false,
		})

	} else {
		// Phase 1: Full Library Metadata Refresh
		rm.set(Progress{Message: "Getting library count...", Running: true})

		// Step 1: Get total count (this is the count of actual Emby items, not codec entries)
		count, err := em.TotalItems()
		if err != nil {
			rm.set(Progress{Error: err.Error(), Done: true})
			return
		}
		total = count
		rm.set(Progress{Total: total, Message: "Fetching library items...", Running: true})

		// Step 2: Fetch library items in chunks
		page := 0
		for actualItemsProcessed < total {
			// GetItemsChunk now returns one entry per media item (1:1 mapping)
			libraryEntries, err := em.GetItemsChunk(chunkSize, page)
			if err != nil {
				rm.set(Progress{Error: err.Error(), Done: true})
				return
			}

			if len(libraryEntries) == 0 {
				break // No more items to process
			}

			// Process library entries
			_ = rm.processLibraryEntries(db, em, libraryEntries)

			// Simple counting now that we have 1:1 mapping
			actualItemsProcessed += len(libraryEntries)

			rm.set(Progress{
				Total:     total,
				Processed: actualItemsProcessed,
				Message:   fmt.Sprintf("Processed %d / %d items", actualItemsProcessed, total),
				Page:      page,
				Running:   true,
			})
			page++
			time.Sleep(100 * time.Millisecond)
		}

		// Update full sync timestamp
		if err := syncpkg.UpdateSyncTime(db, syncpkg.SyncTypeLibraryFull, actualItemsProcessed); err != nil {
			logging.Debug("Failed to update sync timestamp: %v", err)
		}
	}

	// Phase 2: Play History Collection (only for full sync)
	if !incremental {
		rm.set(Progress{
			Total:     total,
			Processed: total,
			Message:   "Library complete! Now collecting play history...",
			Running:   true,
		})

		// Get all users and collect their complete history
		users, err := em.GetUsers()
		if err != nil {
			rm.set(Progress{Error: "Failed to get users for history collection: " + err.Error(), Done: true})
			return
		}

		totalHistoryEvents := 0
		for userIndex, user := range users {
			rm.set(Progress{
				Total:     total,
				Processed: total,
				Message:   fmt.Sprintf("Collecting history for user %s (%d/%d)...", user.Name, userIndex+1, len(users)),
				Running:   true,
			})

			// Get unlimited history for this user (0 = all history)
			history, err := em.GetUserPlayHistory(user.Id, 0)
			if err != nil {
				logging.Debug("Failed to get history for user %s: %v", user.Name, err)
				continue // Skip user but don't fail entire refresh
			}

			userEvents := 0
			for _, h := range history {
				// Upsert user and item info
				_, _ = db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET name=excluded.name`, user.Id, user.Name)
				_, _ = db.Exec(`INSERT INTO library_item (id, server_id, item_id, name, media_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT(id) DO UPDATE SET name=COALESCE(excluded.name, library_item.name), media_type=COALESCE(excluded.media_type, library_item.media_type), updated_at=CURRENT_TIMESTAMP`, h.Id, h.Id, h.Id, h.Name, h.Type)

				// Convert position to milliseconds
				posMs := int64(0)
				if h.PlaybackPos > 0 {
					posMs = h.PlaybackPos / 10_000
				}

				// Use historical timestamp
				var eventTime int64 = time.Now().UnixMilli() // fallback
				if h.DatePlayed != "" {
					if playTime, err := time.Parse(time.RFC3339, h.DatePlayed); err == nil {
						eventTime = playTime.UnixMilli()
					} else if playTime, err := time.Parse("2006-01-02T15:04:05", h.DatePlayed); err == nil {
						eventTime = playTime.UnixMilli()
					}
				}

				// Insert play event
				result, err := db.Exec(`INSERT OR IGNORE INTO play_event (ts, user_id, item_id, pos_ms) VALUES (?, ?, ?, ?)`, eventTime, user.Id, h.Id, posMs)
				if err == nil {
					if rows, _ := result.RowsAffected(); rows > 0 {
						userEvents++
						totalHistoryEvents++
					}
				}
			}

			if userEvents > 0 {
				logging.Debug("User %s: collected %d historical events", user.Name, userEvents)
			}
		}

		// Complete!
		rm.set(Progress{
			Total:     total,
			Processed: total,
			Message:   fmt.Sprintf("Complete! Library: %d items, History: %d events from %d users", actualItemsProcessed, totalHistoryEvents, len(users)),
			Done:      true,
			Running:   false,
		})
	}
}

// processLibraryEntries handles the insertion and enrichment of library items
func (rm *RefreshManager) processLibraryEntries(db *sql.DB, em *emby.Client, libraryEntries []emby.LibraryItem) int {
	dbEntriesInserted := 0
	for _, entry := range libraryEntries {
		// Extract width from height for older data compatibility
		var width *int
		if entry.Height != nil && *entry.Height > 0 {
			// For 16:9 content, width = height * 16/9
			calculatedWidth := int(float64(*entry.Height) * 16.0 / 9.0)
			width = &calculatedWidth
		}

        // Include runtime ticks and container when available
        result, err := db.Exec(`
            INSERT INTO library_item (id, server_id, item_id, name, media_type, height, width, run_time_ticks, container, video_codec, file_size_bytes, bitrate_bps, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
            ON CONFLICT(id) DO UPDATE SET
                server_id = COALESCE(excluded.server_id, library_item.server_id),
                item_id = COALESCE(excluded.item_id, library_item.item_id),
                name = COALESCE(excluded.name, library_item.name),
                media_type = COALESCE(excluded.media_type, library_item.media_type),
                height = COALESCE(excluded.height, library_item.height),
                width = COALESCE(excluded.width, library_item.width),
                run_time_ticks = COALESCE(excluded.run_time_ticks, library_item.run_time_ticks),
                container = COALESCE(excluded.container, library_item.container),
                video_codec = COALESCE(excluded.video_codec, library_item.video_codec),
                file_size_bytes = COALESCE(excluded.file_size_bytes, library_item.file_size_bytes),
                bitrate_bps = COALESCE(excluded.bitrate_bps, library_item.bitrate_bps),
                updated_at = CURRENT_TIMESTAMP
        `, entry.Id, entry.Id, entry.Id, entry.Name, entry.Type, entry.Height, width, entry.RunTimeTicks, entry.Container, entry.Codec, entry.FileSizeBytes, entry.BitrateBps)

		// For episodes, ensure we have proper series info
		if entry.Type == "Episode" && em != nil {
			// Enrich episode data immediately during refresh
            if episodeItems, err := em.ItemsByIDs([]string{entry.Id}); err == nil && len(episodeItems) > 0 {
                ep := episodeItems[0]
                if ep.SeriesName != "" {
                    // Build proper display name
                    display := ep.Name
					if ep.ParentIndexNumber != nil && ep.IndexNumber != nil {
						season := *ep.ParentIndexNumber
						episode := *ep.IndexNumber
						epcode := fmt.Sprintf("S%02dE%02d", season, episode)
						if ep.SeriesName != "" && ep.Name != "" {
							display = fmt.Sprintf("%s - %s (%s)", ep.SeriesName, ep.Name, epcode)
						}
					}
                    // Update the database with enriched info (+ series linkage)
                    db.Exec(`UPDATE library_item SET name = ?, series_id = COALESCE(?, series_id), series_name = COALESCE(?, series_name), updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
                        display, nullIfEmpty(ep.SeriesId), nullIfEmpty(ep.SeriesName), entry.Id)
                }
            }
        }

		if err == nil {
			if rows, _ := result.RowsAffected(); rows > 0 {
				dbEntriesInserted++
			}
		}
	}
	return dbEntriesInserted
}

// helper: convert empty string to nil for COALESCE updates
func nullIfEmpty(s string) any {
    if strings.TrimSpace(s) == "" { return nil }
    return s
}

// StartHandler kicks off a background refresh using the provided chunk size.
func StartHandler(rm *RefreshManager, db *sql.DB, em *emby.Client, chunkSize int) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fire-and-forget
		rm.Start(db, em, chunkSize)
		return c.JSON(fiber.Map{"status": "started"})
	}
}

// StreamHandler sends progress events over SSE until Done=true.
func StreamHandler(rm *RefreshManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		logging.Debug("SSE subscriber connected")
		defer logging.Debug("SSE subscriber disconnected")
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		// Send an initial hello so clients can attach listeners
		if _, err := c.Write([]byte("event: hello\ndata: {}\n\n")); err != nil {
			return nil
		}
		if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
			_ = f.Flush()
		}

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			p := rm.get()
			payload, _ := json.Marshal(p)
			if _, err := c.Write([]byte("event: progress\ndata: " + string(payload) + "\n\n")); err != nil {
				// client disconnected
				return nil
			}
			// best-effort flush (works under Fiber v3's HTTP server)
			if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
				_ = f.Flush()
			}

			if p.Done {
				return nil
			}
			<-ticker.C
		}
	}
}

// FullHandler starts a refresh and streams progress in the same request.
func FullHandler(rm *RefreshManager, db *sql.DB, em *emby.Client, chunkSize int) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Start the job
		rm.Start(db, em, chunkSize)

		// Then stream like StreamHandler
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		if _, err := c.Write([]byte("event: hello\ndata: {}\n\n")); err != nil {
			return nil
		}
		if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
			_ = f.Flush()
		}

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			p := rm.get()
			b, _ := json.Marshal(p)
			if _, err := c.Write([]byte("event: progress\ndata: " + string(b) + "\n\n")); err != nil {
				return nil
			}
			if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
				_ = f.Flush()
			}
			if p.Done {
				return nil
			}
			<-ticker.C
		}
	}
}

// POST /admin/refresh  -> { started: true }
func StartPostHandler(rm *RefreshManager, db *sql.DB, em *emby.Client, chunkSize int) fiber.Handler {
	return func(c fiber.Ctx) error {
		rm.Start(db, em, chunkSize)
		return c.JSON(fiber.Map{"started": true})
	}
}

// POST /admin/refresh/incremental  -> { started: true }
func StartIncrementalHandler(rm *RefreshManager, db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		rm.StartIncremental(db, em)
		return c.JSON(fiber.Map{"started": true, "type": "incremental"})
	}
}

// GET /admin/refresh/status -> { running, imported, total, page, error }
func StatusHandler(rm *RefreshManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		p := rm.get()
		return c.JSON(fiber.Map{
			"running":  p.Running && !p.Done,
			"imported": p.Processed,
			"total":    p.Total,
			"page":     p.Page,
			"error":    ifEmptyNil(p.Error),
		})
	}
}

func ifEmptyNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
