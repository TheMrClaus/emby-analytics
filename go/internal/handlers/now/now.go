package now

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/emby"
)

// NowEntry is what the frontend expects for each card.
type NowEntry struct {
	Timestamp   int64   `json:"timestamp"`
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
	SessionID   string  `json:"session_id"` // used by control buttons

	// references
	ItemID   string `json:"item_id"`
	ItemType string `json:"item_type,omitempty"`
}

// Assume you already have this helper in your project.
func getEmbyClient(c *fiber.Ctx) (*emby.Client, error) {
	// Typically pulled from app config / ctx locals. This is a placeholder.
	// Replace with your existing implementation if different.
	type cfg struct {
		BaseURL string
		APIKey  string
	}
	confAny := c.Locals("emby_config")
	if confAny == nil {
		return nil, fmt.Errorf("emby config not found in context")
	}
	conf := confAny.(cfg)
	return emby.New(conf.BaseURL, conf.APIKey), nil
}

// Snapshot returns the current list once.
func Snapshot(c *fiber.Ctx) error {
	em, err := getEmbyClient(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	sessions, err := em.GetActiveSessions()
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	nowMs := time.Now().UnixMilli()
	out := make([]NowEntry, 0, len(sessions))
	for _, s := range sessions {
		var progressPct float64
		if s.DurationTicks > 0 {
			progressPct = (float64(s.PosTicks) / float64(s.DurationTicks)) * 100.0
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
			SessionID:   s.SessionID,
			ItemID:      s.ItemID,
			ItemType:    s.ItemType,
		})
	}
	return c.JSON(out)
}

// Stream pushes snapshots periodically via SSE (default message events).
func Stream(c *fiber.Ctx) error {
	// SSE/CORS headers (allow cross-origin dashboards).
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	w := bufio.NewWriter(c.Context().Response.BodyWriter())
	flush := func() { _ = w.Flush() }

	em, err := getEmbyClient(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	// initial tick immediately
	send := func() bool {
		sessions, err := em.GetActiveSessions()
		if err != nil {
			log.Printf("[now] get sessions: %v", err)
			return false
		}
		nowMs := time.Now().UnixMilli()
		out := make([]NowEntry, 0, len(sessions))
		for _, s := range sessions {
			var progressPct float64
			if s.DurationTicks > 0 {
				progressPct = (float64(s.PosTicks) / float64(s.DurationTicks)) * 100.0
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
				SessionID:   s.SessionID,
				ItemID:      s.ItemID,
				ItemType:    s.ItemType,
			})
		}
		b, _ := json.Marshal(out)
		if _, err := w.WriteString("data: " + string(b) + "\n\n"); err != nil {
			return false
		}
		flush()
		return true
	}

	// send immediately, then on each tick
	if !send() {
		// keep connection open anyway; client will retry on next tick
	}
	for {
		select {
		case <-c.Context().Done():
			return nil
		case <-ticker.C:
			if !send() {
				// transient error; continue and try next tick
			}
		}
	}
}

// ----- Controls (pause/stop/message) -----

// POST /now/sessions/:id/pause  body: {"paused":true} pauses, {"paused":false} unpauses.
// If body omitted, defaults to pause.
func PauseSession(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Paused *bool `json:"paused"`
	}
	_ = c.BodyParser(&body)

	em, err := getEmbyClient(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if body.Paused != nil && !*body.Paused {
		if err := em.Unpause(id); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
	if err := em.Pause(id); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /now/sessions/:id/stop
func StopSession(c *fiber.Ctx) error {
	id := c.Params("id")
	em, err := getEmbyClient(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := em.Stop(id); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /now/sessions/:id/message  body: {header?, text, timeout_ms?}
func MessageSession(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Header    string `json:"header"`
		Text      string `json:"text"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if strings.TrimSpace(body.Text) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "text required"})
	}

	em, err := getEmbyClient(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	header := body.Header
	if header == "" {
		header = "Emby Analytics"
	}
	if err := em.SendMessage(id, header, body.Text, body.TimeoutMs); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// (Optional) If you register routes here, do something like:
//
//  app.Get("/now", Snapshot)
//  app.Get("/now/stream", Stream)
//  app.Post("/now/sessions/:id/pause", PauseSession)
//  app.Post("/now/sessions/:id/stop", StopSession)
//  app.Post("/now/sessions/:id/message", MessageSession)

// Dummy references so the compiler keeps these imports if your project
// doesn't use sql/log directly here; remove if unneeded.
var _ = sql.ErrNoRows
var _ = log.Printf
