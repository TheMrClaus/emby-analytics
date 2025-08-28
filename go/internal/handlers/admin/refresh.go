package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type Progress struct {
	Total     int    `json:"total"`
	Processed int    `json:"processed"`
	Message   string `json:"message"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
	Page      int    `json:"page"`
	Running   bool   `json:"running"`
}

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

func (rm *RefreshManager) get() Progress {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.progress
}

// Start a background refresh
func (rm *RefreshManager) Start(db *sql.DB, em *emby.Client, chunkSize int) {
	rm.set(Progress{Message: "Starting refresh...", Running: true})
	go rm.refreshWorker(db, em, chunkSize)
}

func (rm *RefreshManager) refreshWorker(db *sql.DB, em *emby.Client, chunkSize int) {
	var total int
	var actualItemsProcessed int

	// Phase 1: Library Metadata Refresh
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

		// Insert library entries into DB (now 1:1 mapping)
		dbEntriesInserted := 0
		for _, entry := range libraryEntries {
			// Extract width from height for older data compatibility
			var width *int
			if entry.Height != nil && *entry.Height > 0 {
				// For 16:9 content, width = height * 16/9
				calculatedWidth := int(float64(*entry.Height) * 16.0 / 9.0)
				width = &calculatedWidth
			}

			result, err := db.Exec(`
				INSERT INTO library_item (id, server_id, item_id, name, media_type, height, width, video_codec, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				ON CONFLICT(id) DO UPDATE SET
					server_id = COALESCE(excluded.server_id, library_item.server_id),
					item_id = COALESCE(excluded.item_id, library_item.item_id),
					name = COALESCE(excluded.name, library_item.name),
					media_type = COALESCE(excluded.media_type, library_item.media_type),
					height = COALESCE(excluded.height, library_item.height),
					width = COALESCE(excluded.width, library_item.width),
					video_codec = COALESCE(excluded.video_codec, library_item.video_codec),
					updated_at = CURRENT_TIMESTAMP
			`, entry.Id, entry.Id, entry.Id, entry.Name, entry.Type, entry.Height, width, entry.Codec)

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
						// Update the database with enriched info
						db.Exec(`UPDATE library_item SET name = ?, media_type = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
							display, "Series", entry.Id)
					}
				}
			}

			if err == nil {
				if rows, _ := result.RowsAffected(); rows > 0 {
					dbEntriesInserted++
				}
			}
		}

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

	// Phase 2: Play History Collection
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
			log.Printf("[refresh] Failed to get history for user %s: %v", user.Name, err)
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
			log.Printf("[refresh] User %s: collected %d historical events", user.Name, userEvents)
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
		log.Println("[admin/refresh] SSE subscriber connected")
		defer log.Println("[admin/refresh] SSE subscriber disconnected")
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
