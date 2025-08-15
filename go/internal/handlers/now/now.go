package now

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type NowEntry struct {
	Timestamp int64   `json:"timestamp"`
	UserID    string  `json:"user_id"`
	UserName  string  `json:"user_name"`
	ItemID    string  `json:"item_id"`
	ItemName  string  `json:"item_name,omitempty"`
	ItemType  string  `json:"item_type,omitempty"`
	PosHours  float64 `json:"pos_hours"`
}

// GET /now — one-shot snapshot
func Snapshot(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		sessions, err := em.GetActiveSessions()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}
		out := make([]NowEntry, 0, len(sessions))
		nowMs := time.Now().UnixMilli()
		for _, s := range sessions {
			posMs := s.PosMs / 10000 // Emby ticks → ms
			out = append(out, NowEntry{
				Timestamp: nowMs,
				UserID:    s.UserID,
				UserName:  s.UserName,
				ItemID:    s.ItemID,
				ItemName:  s.ItemName,
				ItemType:  s.ItemType,
				PosHours:  float64(posMs) / 3_600_000.0,
			})
		}
		return c.JSON(out)
	}
}

// GET /now/stream — SSE; sends "update" events + periodic keepalives
func Stream(db *sql.DB, em *emby.Client, pollSec int) fiber.Handler {
	if pollSec <= 0 {
		pollSec = 5
	}
	return func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		poll := time.NewTicker(time.Duration(pollSec) * time.Second)
		defer poll.Stop()

		keep := time.NewTicker(15 * time.Second)
		defer keep.Stop()

		// send an initial hello so clients attach listeners confidently
		if _, err := c.Write([]byte("event: hello\ndata: {}\n\n")); err != nil {
			return nil
		}
		if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
			_ = f.Flush()
		}

		for {
			select {
			case <-poll.C:
				sessions, err := em.GetActiveSessions()
				if err != nil {
					// surface a transient error as an event; do not close stream
					msg, _ := json.Marshal(map[string]string{"error": err.Error()})
					if _, werr := c.Write([]byte("event: error\ndata: " + string(msg) + "\n\n")); werr != nil {
						return nil
					}
					if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
						_ = f.Flush()
					}
					continue
				}

				nowMs := time.Now().UnixMilli()
				out := make([]NowEntry, 0, len(sessions))
				for _, s := range sessions {
					posMs := s.PosMs / 10000
					out = append(out, NowEntry{
						Timestamp: nowMs,
						UserID:    s.UserID,
						UserName:  s.UserName,
						ItemID:    s.ItemID,
						ItemName:  s.ItemName,
						ItemType:  s.ItemType,
						PosHours:  float64(posMs) / 3_600_000.0,
					})
				}
				b, _ := json.Marshal(out)
				if _, err := c.Write([]byte("event: update\ndata: " + string(b) + "\n\n")); err != nil {
					return nil // client disconnected
				}
				if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
					_ = f.Flush()
				}

			case <-keep.C:
				// keepalive so proxies don’t close the stream
				if _, err := c.Write([]byte("event: keepalive\ndata: {}\n\n")); err != nil {
					return nil
				}
				if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
					_ = f.Flush()
				}
			}
		}
	}
}
