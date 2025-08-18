package now

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	SessionID   string  `json:"session_id"`

	ItemID   string `json:"item_id"`
	ItemType string `json:"item_type,omitempty"`

	// New richer fields (safe for UI to ignore if unused)
	Container   string `json:"container,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	DolbyVision bool   `json:"dolby_vision,omitempty"`
	HDR10       bool   `json:"hdr10,omitempty"`

	AudioLang string `json:"audio_lang,omitempty"`
	AudioCh   int    `json:"audio_ch,omitempty"`

	SubLang  string `json:"sub_lang,omitempty"`
	SubCodec string `json:"sub_codec,omitempty"`

	TransVideoFrom string `json:"trans_video_from,omitempty"`
	TransVideoTo   string `json:"trans_video_to,omitempty"`
	TransAudioFrom string `json:"trans_audio_from,omitempty"`
	TransAudioTo   string `json:"trans_audio_to,omitempty"`
}

// Generic env-based Emby client (keeps things portable behind any proxy).
// Set EMBY_BASE_URL and EMBY_API_KEY in your container/env.
func getEmbyClient() (*emby.Client, error) {
	base := strings.TrimRight(os.Getenv("EMBY_BASE_URL"), "/")
	key := os.Getenv("EMBY_API_KEY")
	if base == "" || key == "" {
		return nil, fmt.Errorf("EMBY_BASE_URL or EMBY_API_KEY not set")
	}
	return emby.New(base, key), nil
}

// Snapshot returns the current list once.
func Snapshot(c fiber.Ctx) error {
	em, err := getEmbyClient()
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

			Container:      s.Container,
			Width:          s.Width,
			Height:         s.Height,
			DolbyVision:    s.DolbyVision,
			HDR10:          s.HDR10,
			AudioLang:      s.AudioLang,
			AudioCh:        s.AudioCh,
			SubLang:        s.SubLang,
			SubCodec:       s.SubCodec,
			TransVideoFrom: s.TransVideoFrom,
			TransVideoTo:   s.TransVideoTo,
			TransAudioFrom: s.TransAudioFrom,
			TransAudioTo:   s.TransAudioTo,
		})
	}
	return c.JSON(out)
}

// Stream pushes snapshots periodically via SSE (default message events).
func Stream(c fiber.Ctx) error {
	// SSE/CORS headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	w := bufio.NewWriter(c.RequestCtx().Response.BodyWriter())
	flush := func() { _ = w.Flush() }

	em, err := getEmbyClient()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

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

				Container:      s.Container,
				Width:          s.Width,
				Height:         s.Height,
				DolbyVision:    s.DolbyVision,
				HDR10:          s.HDR10,
				AudioLang:      s.AudioLang,
				AudioCh:        s.AudioCh,
				SubLang:        s.SubLang,
				SubCodec:       s.SubCodec,
				TransVideoFrom: s.TransVideoFrom,
				TransVideoTo:   s.TransVideoTo,
				TransAudioFrom: s.TransAudioFrom,
				TransAudioTo:   s.TransAudioTo,
			})
		}
		b, _ := json.Marshal(out)
		if _, err := w.WriteString("data: " + string(b) + "\n\n"); err != nil {
			return false
		}
		flush()
		return true
	}

	// initial push, then ticks
	_ = send()
	for {
		select {
		case <-c.Done():
			return nil
		case <-ticker.C:
			_ = send()
		}
	}
}

// ----- Controls (pause/stop/message) -----

// POST /now/sessions/:id/pause  body: {"paused":true} pauses, {"paused":false} unpauses.
// If body omitted, defaults to pause.
func PauseSession(c fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Paused *bool `json:"paused"`
	}
	_ = c.Bind().Body(&body)

	em, err := getEmbyClient()
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
func StopSession(c fiber.Ctx) error {
	id := c.Params("id")
	em, err := getEmbyClient()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := em.Stop(id); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /now/sessions/:id/message  body: {header?, text, timeout_ms?}
func MessageSession(c fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Header    string `json:"header"`
		Text      string `json:"text"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if strings.TrimSpace(body.Text) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "text required"})
	}

	em, err := getEmbyClient()
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

// Router wiring elsewhere should use fiber v3 style:
//   app.Get("/now", Snapshot)
//   app.Get("/now/stream", Stream)
//   app.Post("/now/sessions/:id/pause", PauseSession)
//   app.Post("/now/sessions/:id/stop", StopSession)
//   app.Post("/now/sessions/:id/message", MessageSession)

// Dummy references so the compiler keeps these imports if unneeded here.
var _ = sql.ErrNoRows
var _ = log.Printf
