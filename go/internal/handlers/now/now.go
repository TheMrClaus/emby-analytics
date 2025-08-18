package now

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

type NowEntry struct {
	Timestamp int64 `json:"timestamp"`
	// Fields the UI expects (app/src/pages/index.tsx):
	Title       string  `json:"title"`
	User        string  `json:"user"`
	App         string  `json:"app"`
	Device      string  `json:"device"`
	PlayMethod  string  `json:"play_method"`
	Video       string  `json:"video"`
	Audio       string  `json:"audio"`
	Subs        string  `json:"subs"`
	Bitrate     int64   `json:"bitrate"`
	ProgressPct float64 `json:"progress_pct"`
	Poster      string  `json:"poster"`

	// Useful references
	ItemID   string `json:"item_id"`
	ItemType string `json:"item_type,omitempty"`
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
			// ticks -> ms
			posMs := s.PosTicks / 10_000
			durTicks := s.DurationTicks
			progressPct := 0.0
			if durTicks > 0 {
				progressPct = (float64(s.PosTicks) / float64(durTicks)) * 100.0
				if progressPct < 0 {
					progressPct = 0
				}
				if progressPct > 100 {
					progressPct = 100
				}
			}

			subsText := "None"
			if s.SubsCount > 0 {
				subsText = fmt.Sprintf("%d", s.SubsCount)
			}

			poster := ""
			if s.ItemID != "" {
				poster = "/img/primary/" + s.ItemID
			}

			out = append(out, NowEntry{
				Timestamp:   nowMs,
				Title:       s.ItemName,
				User:        s.UserName,
				App:         s.App,
				Device:      s.Device,
				PlayMethod:  s.PlayMethod,
				Video:       s.VideoCodec,
				Audio:       s.AudioCodec,
				Subs:        subsText,
				Bitrate:     s.Bitrate,
				ProgressPct: progressPct,
				Poster:      poster,
				ItemID:      s.ItemID,
				ItemType:    s.ItemType,
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
					// Send as default message event so EventSource.onmessage receives it
					if _, err := w.WriteString("data: " + string(b) + "\n\n"); err != nil {
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
