package now

import (
	"fmt"
	"strings"
	"time"

	ws "github.com/saveblush/gofiber3-contrib/websocket"

	"emby-analytics/internal/media"
	"context"
)

// MultiWS upgrades to WebSocket and periodically sends aggregated multi-server NowEntry snapshots.
// Supports optional query param ?server=emby|plex|jellyfin|all to filter by server type.
func MultiWS() func(*ws.Conn) {
	return func(conn *ws.Conn) {
		defer conn.Close()

		// Parse filter at connection time
		serverFilter := "all"
		if conn != nil && conn.Params("server") != "" {
			serverFilter = strings.ToLower(conn.Params("server"))
		} else if q := conn.Query("server"); q != "" {
			serverFilter = strings.ToLower(q)
		}

		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()

		send := func() bool {
			entries, err := fetchMultiNowEntries(serverFilter)
			if err != nil {
				// best-effort: send empty payload with error as text for diagnostics
				_ = conn.WriteJSON([]NowEntry{})
				return true
			}
			if err := conn.WriteJSON(entries); err != nil {
				return false
			}
			return true
		}

		// initial send
		if !send() {
			return
		}

		for {
			select {
			case <-ticker.C:
				if !send() {
					return
				}
			}
		}
	}
}

func fetchMultiNowEntries(filter string) ([]NowEntry, error) {
	if multiServerMgr == nil {
		return []NowEntry{}, nil
	}

	// Gather sessions by filter
	var sessions []media.Session
	lf := strings.ToLower(strings.TrimSpace(filter))
	switch lf {
	case "", "all":
		ss, err := multiServerMgr.GetAllSessionsCached(context.Background())
		if err != nil {
			return nil, err
		}
		sessions = ss
	case string(media.ServerTypeEmby), string(media.ServerTypePlex), string(media.ServerTypeJellyfin):
		for _, client := range multiServerMgr.ClientsByType(media.ServerType(lf)) {
			if ss, err := client.GetActiveSessions(); err == nil {
				sessions = append(sessions, ss...)
			}
		}
	default:
		// Unknown alias; return empty
		sessions = []media.Session{}
	}

	nowMs := time.Now().UnixMilli()
	out := make([]NowEntry, 0, len(sessions))
	for _, s := range sessions {
		var progressPct float64
		if s.DurationMs > 0 {
			progressPct = (float64(s.PositionMs) / float64(s.DurationMs)) * 100
			if progressPct < 0 {
				progressPct = 0
			}
			if progressPct > 100 {
				progressPct = 100
			}
		}
		subsText := "None"
		if s.SubtitleCount > 0 {
			subsText = fmt.Sprintf("%d", s.SubtitleCount)
		}
		poster := ""
		if s.ItemID != "" {
			poster = "/img/primary/" + string(s.ServerType) + "/" + s.ItemID
		}

		e := NowEntry{
			Timestamp:      nowMs,
			Title:          s.ItemName,
			User:           s.UserName,
			App:            s.ClientApp,
			Device:         s.DeviceName,
			PlayMethod:     s.PlayMethod,
			Video:          strings.TrimSpace(videoDetailFromNormalized(s)),
			Audio:          strings.TrimSpace(audioDetailFromNormalized(s)),
			Subs:           subsText,
			Bitrate:        s.Bitrate,
			ProgressPct:    progressPct,
			PositionSec:    s.PositionMs / 1000,
			DurationSec:    s.DurationMs / 1000,
			Poster:         poster,
			SessionID:      s.SessionID,
			ItemID:         s.ItemID,
			ItemType:       s.ItemType,
			Container:      s.Container,
			Width:          s.Width,
			Height:         s.Height,
			DolbyVision:    s.DolbyVision,
			HDR10:          s.HDR10,
			AudioLang:      s.AudioLanguage,
			AudioCh:        s.AudioChannels,
			SubLang:        s.SubtitleLanguage,
			SubCodec:       s.SubtitleCodec,
			TransVideoFrom: strings.ToUpper(s.VideoCodec),
			TransVideoTo:   strings.ToUpper(s.TranscodeVideoCodec),
			TransAudioFrom: strings.ToUpper(s.AudioCodec),
			TransAudioTo:   strings.ToUpper(s.TranscodeAudioCodec),
			VideoMethod:    s.VideoMethod,
			AudioMethod:    s.AudioMethod,
			TransReason:    reasonText(s.VideoMethod, s.AudioMethod, s.TranscodeReasons),
			TransPct: func() float64 {
				if s.TranscodeProgress > 0 {
					return s.TranscodeProgress
				}
				if s.DurationMs > 0 {
					return (float64(s.PositionMs) / float64(s.DurationMs)) * 100
				}
				return 0
			}(),
			ServerID:   s.ServerID,
			ServerType: string(s.ServerType),
		}
		if strings.EqualFold(s.PlayMethod, "Transcode") {
			e.StreamPath = streamPathLabel(s.TranscodeContainer)
			e.StreamDetail = fmt.Sprintf("%s (%s)", e.StreamPath, mbps(s.Bitrate))
			e.TransVideoBitrate = s.TranscodeBitrate
		}
		out = append(out, e)
	}
	return out, nil
}
