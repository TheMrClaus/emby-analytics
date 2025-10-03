package now

import (
	"bufio"
	"database/sql"
	"emby-analytics/internal/logging"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
	// New: explicit time fields for nicer UI formatting
	PositionSec int64  `json:"position_sec,omitempty"`
	DurationSec int64  `json:"duration_sec,omitempty"`
	Poster      string `json:"poster"`
	SessionID   string `json:"session_id"`

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
	VideoMethod string `json:"video_method,omitempty"` // "Direct Play"/"Transcode"
	AudioMethod string `json:"audio_method,omitempty"`

	StreamPath   string `json:"stream_path,omitempty"`   // e.g. HLS
	StreamDetail string `json:"stream_detail,omitempty"` // e.g. "HLS (6.2 Mbps, 1483 fps)"
	TransReason  string `json:"trans_reason,omitempty"`

	// For the red transcode bar
	TransPct float64 `json:"trans_pct,omitempty"`

	// For parentheses after lines when transcoding
	TransAudioBitrate int64 `json:"trans_audio_bitrate,omitempty"`
	TransVideoBitrate int64 `json:"trans_video_bitrate,omitempty"`

	// Playback state
	IsPaused bool `json:"is_paused,omitempty"`

	// Server metadata (for multi-server UI)
	ServerID   string `json:"server_id,omitempty"`
	ServerType string `json:"server_type,omitempty"`
	SeriesID   string `json:"series_id,omitempty"`
}

// sanitizeMessageInput cleans user input to prevent injection attacks
func sanitizeMessageInput(input string, maxLength int) string {
	if input == "" {
		return ""
	}

	// Step 1: Trim whitespace
	input = strings.TrimSpace(input)

	// Step 2: Enforce length limit
	if len(input) > maxLength {
		// Use utf8 aware truncation
		if utf8.ValidString(input) {
			runes := []rune(input)
			if len(runes) > maxLength {
				input = string(runes[:maxLength])
			}
		} else {
			// Fallback for invalid UTF-8
			input = input[:maxLength]
		}
	}

	// Step 3: HTML escape to prevent script injection
	input = html.EscapeString(input)

	// Step 4: Remove dangerous patterns that might still cause issues
	// Remove any remaining script/HTML-like patterns (belt and suspenders)
	patterns := []string{
		`<script[^>]*>.*?</script>`,
		`<iframe[^>]*>.*?</iframe>`,
		`<object[^>]*>.*?</object>`,
		`<embed[^>]*>`,
		`<form[^>]*>.*?</form>`,
		`javascript:`,
		`vbscript:`,
		`data:text/html`,
		`<meta[^>]*>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		input = re.ReplaceAllString(input, "")
	}

	// Step 5: Remove control characters except common ones (tab, newline)
	var cleaned strings.Builder
	for _, r := range input {
		// Allow normal printable chars, space, tab, newline
		if r >= 32 || r == '\t' || r == '\n' || r == '\r' {
			cleaned.WriteRune(r)
		}
	}

	return cleaned.String()
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
			if s.ItemType == "Episode" && s.SeriesID != "" {
				poster = "/img/primary/" + s.SeriesID
			} else {
				poster = "/img/primary/" + s.ItemID
			}
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
			PositionSec: func() int64 {
				if s.PosTicks > 0 {
					return s.PosTicks / 10_000_000
				}
				return 0
			}(),
			DurationSec: func() int64 {
				if s.DurationTicks > 0 {
					return s.DurationTicks / 10_000_000
				}
				return 0
			}(),
			Poster:    poster,
			SessionID: s.SessionID,
			ItemID:    s.ItemID,
			ItemType:  s.ItemType,
			SeriesID:  s.SeriesID,

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

			IsPaused: s.IsPaused,
		})
	}
	return c.JSON(out)
}

// Stream pushes snapshots periodically via SSE (default message events).
func Stream(c fiber.Ctx) error {
	logging.Debug("SSE client connected from %s", c.IP())

	// SSE/CORS headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")

	w := bufio.NewWriter(c.RequestCtx().Response.BodyWriter())
	flush := func() error {
		if err := w.Flush(); err != nil {
			return err
		}
		// Force immediate response flush in Fiber v3
		if f, ok := c.Response().BodyWriter().(interface{ Flush() error }); ok {
			return f.Flush()
		}
		return nil
	}

	em, err := getEmbyClient()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	dataTicker := time.NewTicker(1500 * time.Millisecond)
	keepaliveTicker := time.NewTicker(10 * time.Second)
	defer func() {
		dataTicker.Stop()
		keepaliveTicker.Stop()
		logging.Debug("SSE client disconnected from %s", c.IP())
	}()

	// Send initial connection event
	if _, err := w.WriteString("event: connected\ndata: {\"status\":\"connected\"}\n\n"); err != nil {
		return nil
	}
	if err := flush(); err != nil {
		return nil
	}

	send := func() bool {
		sessions, err := em.GetActiveSessions()
		if err != nil {
			logging.Debug("get sessions: %v", err)
			// Send error event to client but continue
			if _, err := w.WriteString("event: error\ndata: {\"error\":\"Failed to fetch sessions\"}\n\n"); err != nil {
				return false
			}
			return flush() == nil
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
				if s.ItemType == "Episode" && s.SeriesID != "" {
					poster = "/img/primary/" + s.SeriesID
				} else {
					poster = "/img/primary/" + s.ItemID
				}
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
				PositionSec: func() int64 {
					if s.PosTicks > 0 {
						return s.PosTicks / 10_000_000
					}
					return 0
				}(),
				DurationSec: func() int64 {
					if s.DurationTicks > 0 {
						return s.DurationTicks / 10_000_000
					}
					return 0
				}(),
				Poster:    poster,
				SessionID: s.SessionID,
				ItemID:    s.ItemID,
				ItemType:  s.ItemType,
				SeriesID:  s.SeriesID,

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
				IsPaused:          s.IsPaused,
			})
		}
		b, _ := json.Marshal(out)
		if _, err := w.WriteString("data: " + string(b) + "\n\n"); err != nil {
			return false
		}
		return flush() == nil
	}

	sendKeepalive := func() bool {
		if _, err := w.WriteString("event: keepalive\ndata: {\"timestamp\":" + fmt.Sprintf("%d", time.Now().UnixMilli()) + "}\n\n"); err != nil {
			return false
		}
		return flush() == nil
	}

	// Send initial data
	if !send() {
		return nil
	}

	for {
		select {
		case <-c.Done():
			return nil
		case <-dataTicker.C:
			if !send() {
				return nil
			}
		case <-keepaliveTicker.C:
			if !sendKeepalive() {
				return nil
			}
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
// POST /now/sessions/:id/message  body: {header?, text, timeout_ms?}
func MessageSession(c fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Header string `json:"header"`
		Text   string `json:"text"`
		// Accept alternate field name for convenience
		Message   string `json:"message"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON body"})
	}

	// If client sent {message: "..."}, treat as text
	if strings.TrimSpace(body.Text) == "" && strings.TrimSpace(body.Message) != "" {
		body.Text = body.Message
	}

	// Sanitize inputs
	const maxHeaderLength = 100
	const maxTextLength = 500

	body.Header = sanitizeMessageInput(body.Header, maxHeaderLength)
	body.Text = sanitizeMessageInput(body.Text, maxTextLength)

	// Validate sanitized text
	if strings.TrimSpace(body.Text) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Message text required"})
	}

	// Validate timeout
	if body.TimeoutMs < 1000 {
		body.TimeoutMs = 5000 // Default 5 seconds
	}
	if body.TimeoutMs > 60000 {
		body.TimeoutMs = 60000 // Max 60 seconds
	}

	em, err := getEmbyClient()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Set safe default header if empty after sanitization
	if body.Header == "" {
		body.Header = "Emby Analytics"
	}

	// Log the message attempt for security monitoring
	logging.Debug("[SECURITY] Message sent to session %s: header='%s' text='%s'",
		id, body.Header, body.Text)

	if err := em.SendMessage(id, body.Header, body.Text, body.TimeoutMs); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Dummy references so the compiler keeps these imports if unneeded here.
var _ = sql.ErrNoRows
var _ = logging.Debug
