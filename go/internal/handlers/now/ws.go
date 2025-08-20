package now

import (
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
}
