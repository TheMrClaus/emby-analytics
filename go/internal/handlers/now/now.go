package now

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"log"
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

// Snapshot: GET /now
func Snapshot(db *sql.DB, em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		sessions, err := em.GetActiveSessions()
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
		}
		nowMs := time.Now().UnixMilli()
		out := make([]NowEntry, 0, len(sessions))
		for _, s := range sessions {
			posMs := s.PosMs / 10_000 // ticks -> ms
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

// Stream: GET /now/stream
func Stream(db *sql.DB, em *emby.Client, pollSec int) fiber.Handler {
	if pollSec <= 0 {
		pollSec = 5
	}
	return func(c fiber.Ctx) error {
		// Log a new subscriber (best-effort client IP)
		ip := c.IP()
		if ip == "" {
			ip = "-"
		}
		log.Printf("[now] SSE subscriber connected from %s", ip)
		defer log.Printf("[now] SSE subscriber from %s disconnected", ip)
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		// NB: No compression for SSE
		c.Response().Header.Del(fiber.HeaderContentEncoding)

		poll := time.NewTicker(time.Duration(pollSec) * time.Second)
		defer poll.Stop()
		keep := time.NewTicker(15 * time.Second)
		defer keep.Stop()

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// initial hello
			_, _ = w.WriteString("event: hello\ndata: {}\n\n")
			_ = w.Flush()

			for {
				select {
				case <-poll.C:
					sessions, err := em.GetActiveSessions()
					if err != nil {
						// send transient error and keep the stream open
						b, _ := json.Marshal(map[string]string{"error": err.Error()})
						if _, err := w.WriteString("event: error\ndata: " + string(b) + "\n\n"); err != nil {
							return // client closed
						}
						if err := w.Flush(); err != nil {
							return
						}
						continue
					}

					nowMs := time.Now().UnixMilli()
					out := make([]NowEntry, 0, len(sessions))
					for _, s := range sessions {
						posMs := s.PosMs / 10_000
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
					if _, err := w.WriteString("event: update\ndata: " + string(b) + "\n\n"); err != nil {
						return
					}
					if err := w.Flush(); err != nil {
						return
					}

				case <-keep.C:
					_, _ = w.WriteString("event: keepalive\ndata: {}\n\n")
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		})
	}
}
