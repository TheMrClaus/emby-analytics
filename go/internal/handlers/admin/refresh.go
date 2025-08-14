package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	rm.set(Progress{Message: "Starting refresh..."})
	go rm.refreshWorker(db, em, chunkSize)
}

func (rm *RefreshManager) refreshWorker(db *sql.DB, em *emby.Client, chunkSize int) {
	var total int
	var processed int

	// Step 1: Get total count
	count, err := em.TotalItems()
	if err != nil {
		rm.set(Progress{Error: err.Error(), Done: true})
		return
	}
	total = count
	rm.set(Progress{Total: total, Message: "Fetching items..."})

	// Step 2: Fetch in chunks
	page := 0
	for processed < total {
		items, err := em.GetItemsChunk(chunkSize, page)
		if err != nil {
			rm.set(Progress{Error: err.Error(), Done: true})
			return
		}
		// Insert into DB
		for _, it := range items {
			_, _ = db.Exec(`
				INSERT INTO library_item (id, name, type, height, codec)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					name=excluded.name,
					type=excluded.type,
					height=excluded.height,
					codec=excluded.codec
			`, it.Id, it.Name, it.Type, it.Height, it.Codec)
		}
		processed += len(items)
		rm.set(Progress{
			Total:     total,
			Processed: processed,
			Message:   fmt.Sprintf("Processed %d / %d", processed, total),
		})
		page++
		time.Sleep(100 * time.Millisecond) // avoid hammering API
	}

	rm.set(Progress{Total: total, Processed: processed, Done: true, Message: "Refresh complete"})
}

// HTTP handler to start refresh
func StartHandler(rm *RefreshManager, db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		rm.Start(db, em, 200)
		return c.JSON(fiber.Map{"status": "started"})
	}
}

// SSE stream
func StreamHandler(rm *RefreshManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			p := rm.get()
			data, _ := json.Marshal(p)
			if _, err := c.Write([]byte(fmt.Sprintf("event: progress\ndata: %s\n\n", data))); err != nil {
				return nil // client disconnected
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

func FullHandler(rm *RefreshManager, db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Start background job
		rm.Start(db, em, 200)

		// Then stream until done
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			p := rm.get()
			data, _ := json.Marshal(p)
			if _, err := c.Write([]byte(fmt.Sprintf("event: progress\ndata: %s\n\n", data))); err != nil {
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
