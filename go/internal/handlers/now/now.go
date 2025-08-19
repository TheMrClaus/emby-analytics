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

	// UI-friendly extras
	VideoMethod  string `json:"video_method,omitempty"` // "Direct Play"/"Transcode"
	AudioMethod  string `json:"audio_method,omitempty"`
	StreamPath   string `json:"stream_path,omitempty"`   // e.g. HLS
	StreamDetail string `json:"stream_detail,omitempty"` // e.g. "HLS (6.2 Mbps, 1483 fps)"
	TransReason  string `json:"trans_reason,omitempty"`

	// For the red transcode bar
	TransPct float64 `json:"trans_pct,omitempty"`

	// For parentheses after lines when transcoding
	TransAudioBitrate int64 `json:"trans_audio_bitrate,omitempty"`
	TransVideoBitrate int64 `json:"trans_video_bitrate,omitempty"`
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

// videoDetailFromSession builds strings like "4K Dolby Vision HEVC"
func videoDetailFromSession(s emby.EmbySession) string {
	parts := []string{}

	// Resolution
	if s.Height >= 2160 {
		parts = append(parts, "4K")
	} else if s.Height >= 1440 {
		parts = append(parts, "1440p")
	} else if s.Height >= 1080 {
		parts = append(parts, "1080p")
	} else if s.Height >= 720 {
		parts = append(parts, "720p")
	}

	// HDR / DV
	if s.DolbyVision {
		parts = append(parts, "Dolby Vision")
	} else if s.HDR10 {
		parts = append(parts, "HDR")
	}

	// Codec
	if s.VideoCodec != "" {
		parts = append(parts, strings.ToUpper(s.VideoCodec))
	}

	if len(parts) == 0 {
		return s.VideoCodec // fallback
	}
	return strings.Join(parts, " ")
}

// audioDetailFromSession builds strings like "English AC3 5.1 (Default)"
func audioDetailFromSession(s emby.EmbySession) string {
	parts := []string{}
	if s.AudioLang != "" {
		parts = append(parts, s.AudioLang) // keep casing from Emby
	}
	if s.AudioCodec != "" {
		parts = append(parts, strings.ToUpper(s.AudioCodec))
	}
	if s.AudioCh > 0 {
		ch := ""
		switch s.AudioCh {
		case 1:
			ch = "1.0"
		case 2:
			ch = "2.0"
		case 3:
			ch = "2.1"
		case 4:
			ch = "4.0"
		case 5:
			ch = "5.0"
		case 6:
			ch = "5.1"
		case 7:
			ch = "6.1"
		case 8:
			ch = "7.1"
		default:
			ch = fmt.Sprintf("%d.0", s.AudioCh)
		}
		parts = append(parts, ch)
	}
	out := strings.TrimSpace(strings.Join(parts, " "))
	if s.AudioDefault {
		if out == "" {
			return "(Default)"
		}
		return out + " (Default)"
	}
	if out == "" {
		return s.AudioCodec
	}
	return out
}

// Map container -> streaming path label
func streamPathLabel(container string) string {
	c := strings.ToLower(container)
	switch c {
	case "ts", "mpegts", "hls", "fmp4":
		return "HLS"
	case "dash":
		return "DASH"
	default:
		if c == "" {
			return "Transcode"
		}
		return strings.ToUpper(container)
	}
}

func mbps(bps int64) string {
	if bps <= 0 {
		return "—"
	}
	f := float64(bps) / 1_000_000.0
	return fmt.Sprintf("%.1f Mbps", f)
}

// Human text for primary reason (borrowed from Emby wording)
func reasonText(videoMethod, audioMethod string, reasons []string) string {
	if len(reasons) == 0 {
		// fallback: infer from which track is transcoding
		switch {
		case videoMethod == "Transcode" && audioMethod == "Transcode":
			return "Converting video and audio to compatible codecs"
		case videoMethod == "Transcode":
			return "Converting video to compatible codec"
		case audioMethod == "Transcode":
			return "Converting audio to compatible codec"
		default:
			return ""
		}
	}
	rset := map[string]bool{}
	for _, r := range reasons {
		rset[strings.ToLower(r)] = true
	}
	switch {
	case rset["audiocodecnotsupported"]:
		return "Converting audio to compatible codec"
	case rset["videocodecnotsupported"]:
		return "Converting video to compatible codec"
	case rset["containernotsupported"]:
		// often remux/HLS with copy for video
		if audioMethod == "Transcode" && videoMethod != "Transcode" {
			return "Remuxing stream; converting audio to compatible codec"
		}
		return "Remuxing stream to a compatible container"
	case rset["subtitlecodecnotsupported"]:
		return "Burning/transforming subtitles to compatible format"
	default:
		// generic
		if audioMethod == "Transcode" && videoMethod != "Transcode" {
			return "Converting audio to compatible codec"
		}
		if videoMethod == "Transcode" && audioMethod != "Transcode" {
			return "Converting video to compatible codec"
		}
		return "Transcoding stream"
	}
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
			Video:       videoDetailFromSession(s),
			Audio:       audioDetailFromSession(s),
			Subs:        subsText,
			Bitrate:     s.Bitrate,
			ProgressPct: progressPct,
			Poster:      poster,
			SessionID:   s.SessionID,
			ItemID:      s.ItemID,
			ItemType:    s.ItemType,

			Container: s.Container,

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

			// UI-friendly extras
			VideoMethod: s.VideoMethod,
			AudioMethod: s.AudioMethod,

			StreamPath: streamPathLabel(s.TransContainer),
			StreamDetail: func() string {
				if !strings.EqualFold(s.PlayMethod, "Transcode") {
					return ""
				}
				fp := ""
				if s.TransFramerate > 0 {
					fp = fmt.Sprintf(", %.0f fps", s.TransFramerate)
				}
				return fmt.Sprintf("%s (%s%s)", streamPathLabel(s.TransContainer), mbps(s.Bitrate), fp)
			}(),
			TransReason: reasonText(s.VideoMethod, s.AudioMethod, s.TransReasons),

			TransPct: func() float64 {
				// Prefer server-reported progress; else fall back to current playback pct
				if s.TransCompletion > 0 {
					if s.TransCompletion > 100 {
						return 100
					}
					return s.TransCompletion
				}
				if s.TransPosTicks > 0 && s.DurationTicks > 0 {
					p := (float64(s.TransPosTicks) / float64(s.DurationTicks)) * 100
					if p > 100 {
						p = 100
					}
					return p
				}
				// fallback: match playback progress (at least shows a bar)
				if s.DurationTicks > 0 {
					p := (float64(s.PosTicks) / float64(s.DurationTicks)) * 100
					if p > 100 {
						p = 100
					}
					return p
				}
				return 0
			}(),

			// expose targets/bitrates once (no duplicates)
			TransAudioBitrate: s.TransAudioBitrate,
			TransVideoBitrate: s.TransVideoBitrate,
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
				Video:       videoDetailFromSession(s),
				Audio:       audioDetailFromSession(s),
				Subs:        subsText,
				Bitrate:     s.Bitrate,
				ProgressPct: progressPct,
				Poster:      poster,
				SessionID:   s.SessionID,
				ItemID:      s.ItemID,
				ItemType:    s.ItemType,

				Container: s.Container,

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

				// UI-friendly extras
				VideoMethod: s.VideoMethod,
				AudioMethod: s.AudioMethod,

				StreamPath: streamPathLabel(s.TransContainer),
				StreamDetail: func() string {
					if !strings.EqualFold(s.PlayMethod, "Transcode") {
						return ""
					}
					fp := ""
					if s.TransFramerate > 0 {
						fp = fmt.Sprintf(", %.0f fps", s.TransFramerate)
					}
					return fmt.Sprintf("%s (%s%s)", streamPathLabel(s.TransContainer), mbps(s.Bitrate), fp)
				}(),
				TransReason: reasonText(s.VideoMethod, s.AudioMethod, s.TransReasons),

				TransPct: func() float64 {
					if s.TransCompletion > 0 {
						if s.TransCompletion > 100 {
							return 100
						}
						return s.TransCompletion
					}
					if s.TransPosTicks > 0 && s.DurationTicks > 0 {
						p := (float64(s.TransPosTicks) / float64(s.DurationTicks)) * 100
						if p > 100 {
							p = 100
						}
						return p
					}
					if s.DurationTicks > 0 {
						p := (float64(s.PosTicks) / float64(s.DurationTicks)) * 100
						if p > 100 {
							p = 100
						}
						return p
					}
					return 0
				}(),

				// expose targets/bitrates once (no duplicates)
				TransAudioBitrate: s.TransAudioBitrate,
				TransVideoBitrate: s.TransVideoBitrate,
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
