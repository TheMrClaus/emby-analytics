package now

import (
    "strings"
    "time"

    "github.com/gofiber/fiber/v3"

    "emby-analytics/internal/media"
)

// multiServerMgr holds the global multi-server manager for handlers
var multiServerMgr *media.MultiServerManager

// SetMultiServerManager sets the manager for multi-server handlers
func SetMultiServerManager(mgr *media.MultiServerManager) {
    multiServerMgr = mgr
}

// MultiSnapshot aggregates sessions from all enabled servers.
// Optional query: ?server=<server_id> to filter by server.
func MultiSnapshot(c fiber.Ctx) error {
    if multiServerMgr == nil {
        return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "multi-server not initialized"})
    }

    serverFilter := strings.TrimSpace(c.Query("server"))
    sessions := make([]media.Session, 0)

    if serverFilter != "" && serverFilter != "all" {
        if client, ok := multiServerMgr.GetClient(serverFilter); ok {
            ss, err := client.GetActiveSessions()
            if err == nil {
                sessions = ss
            }
        }
    } else {
        ss, _ := multiServerMgr.GetAllSessions()
        sessions = ss
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
            subsText = "1"
        }
        poster := ""
        if s.ItemID != "" {
            poster = "/img/primary/" + s.ItemID
        }

        entry := NowEntry{
            Timestamp:   nowMs,
            Title:       s.ItemName,
            User:        s.UserName,
            App:         s.ClientApp,
            Device:      s.DeviceName,
            PlayMethod:  s.PlayMethod,
            Video:       strings.TrimSpace(videoDetailFromNormalized(s)),
            Audio:       strings.TrimSpace(audioDetailFromNormalized(s)),
            Subs:        subsText,
            Bitrate:     s.Bitrate,
            ProgressPct: progressPct,
            PositionSec: s.PositionMs / 1000,
            DurationSec: s.DurationMs / 1000,
            Poster:      poster,
            SessionID:   s.SessionID,
            ItemID:      s.ItemID,
            ItemType:    s.ItemType,
            Container:   s.Container,
            Width:       s.Width,
            Height:      s.Height,
            DolbyVision: s.DolbyVision,
            HDR10:       s.HDR10,
            AudioLang:   s.AudioLanguage,
            AudioCh:     s.AudioChannels,
            SubLang:     s.SubtitleLanguage,
            SubCodec:    s.SubtitleCodec,
            // Transcode details
            TransVideoFrom: strings.ToUpper(s.VideoCodec),
            TransVideoTo:   strings.ToUpper(s.TranscodeVideoCodec),
            TransAudioFrom: strings.ToUpper(s.AudioCodec),
            TransAudioTo:   strings.ToUpper(s.TranscodeAudioCodec),
            VideoMethod:    s.VideoMethod,
            AudioMethod:    s.AudioMethod,
            TransReason:    reasonText(s.VideoMethod, s.AudioMethod, s.TranscodeReasons),
            TransPct:       s.TranscodeProgress,
            IsPaused:       s.IsPaused,
        }
        // Server metadata for UI filtering/coloring
        entry.ServerID = s.ServerID
        entry.ServerType = string(s.ServerType)
        out = append(out, entry)
    }
    return c.JSON(out)
}

// Helpers mapping normalized session to UI strings
func videoDetailFromNormalized(s media.Session) string {
    parts := []string{}
    if s.Height >= 2160 {
        parts = append(parts, "4K")
    } else if s.Height >= 1440 {
        parts = append(parts, "1440p")
    } else if s.Height >= 1080 {
        parts = append(parts, "1080p")
    } else if s.Height >= 720 {
        parts = append(parts, "720p")
    }
    if s.DolbyVision {
        parts = append(parts, "Dolby Vision")
    } else if s.HDR10 {
        parts = append(parts, "HDR")
    }
    if s.VideoCodec != "" {
        parts = append(parts, strings.ToUpper(s.VideoCodec))
    }
    return strings.Join(parts, " ")
}

func audioDetailFromNormalized(s media.Session) string {
    parts := []string{}
    if s.AudioLanguage != "" {
        parts = append(parts, s.AudioLanguage)
    }
    if s.AudioCodec != "" {
        parts = append(parts, strings.ToUpper(s.AudioCodec))
    }
    if s.AudioChannels > 0 {
        ch := ""
        switch s.AudioChannels {
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
            ch = ""
        }
        if ch != "" {
            parts = append(parts, ch)
        }
    }
    return strings.TrimSpace(strings.Join(parts, " "))
}

