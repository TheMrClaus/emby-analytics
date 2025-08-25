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

	// Step 1: Get total count (this is the count of actual Emby items, not codec entries)
	count, err := em.TotalItems()
	if err != nil {
		rm.set(Progress{Error: err.Error(), Done: true})
		return
	}
	total = count
	rm.set(Progress{Total: total, Message: "Fetching items...", Running: true})

	// Step 2: Fetch in chunks
	page := 0
	for actualItemsProcessed < total {
		// GetItemsChunk now returns multiple entries per item (one per codec)
		codecEntries, err := em.GetItemsChunk(chunkSize, page)
		if err != nil {
			rm.set(Progress{Error: err.Error(), Done: true})
			return
		}

		if len(codecEntries) == 0 {
			break // No more items to process
		}

		// Insert codec entries into DB
		dbEntriesInserted := 0
		for _, entry := range codecEntries {
			result, err := db.Exec(`
				INSERT INTO library_item (id, name, type, height, codec)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					name=excluded.name,
					type=excluded.type,
					height=excluded.height,
					codec=excluded.codec
			`, entry.Id, entry.Name, entry.Type, entry.Height, entry.Codec)
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
						db.Exec(`UPDATE library_item SET name = ?, type = ? WHERE id = ?`,
							display, "Series", entry.Id)
					}
				}
			}
			if err == nil {
				if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
					dbEntriesInserted++
				}
			}
		}

		// Count unique actual items processed (not codec entries)
		uniqueItems := make(map[string]bool)
		for _, entry := range codecEntries {
			// Extract original item ID (remove codec suffix)
			originalID := entry.Id
			if len(originalID) > 4 {
				// Remove "_v_codec" or "_a_codec" suffix to get original ID
				if pos := len(originalID) - 1; pos > 0 {
					for i := pos; i >= 0; i-- {
						if originalID[i] == '_' {
							// Check if this looks like our codec suffix pattern
							if i > 0 && (originalID[i-1] == 'v' || originalID[i-1] == 'a') && i >= 2 && originalID[i-2] == '_' {
								originalID = originalID[:i-2]
								break
							}
						}
					}
				}
			}
			uniqueItems[originalID] = true
		}

		actualItemsProcessed += len(uniqueItems)

		rm.set(Progress{
			Total:     total,
			Processed: actualItemsProcessed,
			Message:   fmt.Sprintf("Processed %d / %d items (%d codec entries)", actualItemsProcessed, total, dbEntriesInserted),
			Page:      page,
			Running:   true,
		})
		page++
		time.Sleep(100 * time.Millisecond)
	}

	rm.set(Progress{
		Total:     total,
		Processed: actualItemsProcessed,
		Done:      true,
		Message:   "Refresh complete",
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
