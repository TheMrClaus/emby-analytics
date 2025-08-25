package now

import (
	"fmt"
	"strings"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

// WS returns a Fiber v3 handler that upgrades to WebSocket and streams Now Playing snapshots.
func WS() fiber.Handler {
	return ws.New(func(conn *ws.Conn) {
		defer conn.Close()

		cfg := config.Load()
		em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)

		interval := time.Duration(cfg.NowPollSec) * time.Second
		if interval <= 0 {
			interval = 5 * time.Second
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Send immediately on connect
		if !writeSnapshot(conn, em) {
			return
		}

		for range ticker.C {
			if !writeSnapshot(conn, em) {
				return
			}
		}
	})
}

func writeSnapshot(conn *ws.Conn, em *emby.Client) bool {
	sessions, err := em.GetActiveSessions()
	if err != nil {
		// On error: send empty list so UI just shows "no one playing"
		if err := conn.WriteJSON([]NowEntry{}); err != nil {
			return false
		}
		return true
	}

	now := time.Now().UnixMilli()
	entries := make([]NowEntry, 0, len(sessions))

	for _, s := range sessions {
		var pct float64
		if s.DurationTicks > 0 {
			pct = float64(s.PosTicks) / float64(s.DurationTicks) * 100
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
		}

		entries = append(entries, NowEntry{
			Timestamp:         now,
			Title:             s.ItemName,
			User:              s.UserName,
			App:               s.App,
			Device:            s.Device,
			PlayMethod:        s.PlayMethod,
			Video:             videoDetailFromSession(s),
			Audio:             audioDetailFromSession(s),
			Subs:              s.SubLang,
			Bitrate:           s.Bitrate,
			ProgressPct:       pct,
			Poster:            "/img/primary/" + s.ItemID,
			SessionID:         s.SessionID,
			ItemID:            s.ItemID,
			ItemType:          s.ItemType,
			Container:         s.Container,
			Width:             s.Width,
			Height:            s.Height,
			DolbyVision:       s.DolbyVision,
			HDR10:             s.HDR10,
			AudioLang:         s.AudioLang,
			AudioCh:           s.AudioCh,
			SubLang:           s.SubLang,
			SubCodec:          s.SubCodec,
			StreamPath:        streamPathLabel(s.Container),
			StreamDetail:      mbps(s.Bitrate),
			TransReason:       reasonText(s.VideoMethod, s.AudioMethod, s.TransReasons),
			TransPct:          s.TransCompletion,
			TransAudioBitrate: s.TransAudioBitrate,
			TransVideoBitrate: s.TransVideoBitrate,
			TransVideoFrom:    s.TransVideoFrom,
			TransVideoTo:      s.TransVideoTo,
			TransAudioFrom:    s.TransAudioFrom,
			TransAudioTo:      s.TransAudioTo,
			VideoMethod:       s.VideoMethod,
			AudioMethod:       s.AudioMethod,
		})
	}

	// Send the array directly (no wrapper object!)
	if err := conn.WriteJSON(entries); err != nil {
		return false // client disconnected
	}
	return true
}

// Helper functions (copied from now.go to avoid circular imports)

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
