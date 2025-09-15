package now

import (
    "fmt"
    "strconv"
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
    serverFilter := strings.TrimSpace(c.Query("server"))
    sessions := make([]media.Session, 0)

    if multiServerMgr != nil {
        lf := strings.ToLower(serverFilter)
        switch lf {
        case "", "all":
            if ss, err := multiServerMgr.GetAllSessions(); err == nil {
                sessions = ss
            }
        case string(media.ServerTypeEmby), string(media.ServerTypePlex), string(media.ServerTypeJellyfin):
            // Filter by server type
            for _, client := range multiServerMgr.GetEnabledClients() {
                if client != nil && strings.EqualFold(string(client.GetServerType()), lf) {
                    if ss, err := client.GetActiveSessions(); err == nil {
                        sessions = append(sessions, ss...)
                    }
                }
            }
        default:
            // Treat as server ID
            if client, ok := multiServerMgr.GetClient(serverFilter); ok && client != nil {
                if ss, err := client.GetActiveSessions(); err == nil {
                    sessions = ss
                }
            }
        }
    }

    // Fallback: if no sessions and no specific non-Emby filter, try legacy Emby snapshot
    if len(sessions) == 0 {
        lf := strings.ToLower(serverFilter)
        if lf != "" && lf != "all" && lf != string(media.ServerTypeEmby) && lf != "default-emby" {
            // Specific non-Emby filter requested; return empty
            return c.JSON([]NowEntry{})
        }
        if em, err := getEmbyClient(); err == nil {
            if es, err2 := em.GetActiveSessions(); err2 == nil && len(es) > 0 {
                nowMs := time.Now().UnixMilli()
                out := make([]NowEntry, 0, len(es))
                for _, s := range es {
                    var progressPct float64
                    if s.DurationTicks > 0 {
                        progressPct = (float64(s.PosTicks) / float64(s.DurationTicks)) * 100.0
                        if progressPct < 0 { progressPct = 0 }
                        if progressPct > 100 { progressPct = 100 }
                    }
                    subsText := "None"
                    if s.SubsCount > 0 { subsText = "1" }
                    poster := ""
                    if s.ItemID != "" { poster = "/img/primary/" + s.ItemID }
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
                        PositionSec: func() int64 { if s.PosTicks > 0 { return s.PosTicks / 10_000_000 }; return 0 }(),
                        DurationSec: func() int64 { if s.DurationTicks > 0 { return s.DurationTicks / 10_000_000 }; return 0 }(),
                        Poster:      poster,
                        SessionID:   s.SessionID,
                        ItemID:      s.ItemID,
                        ItemType:    s.ItemType,
                        Container:   s.Container,
                        Width:       s.Width,
                        Height:      s.Height,
                        DolbyVision: s.DolbyVision,
                        HDR10:       s.HDR10,
                        AudioLang:   s.AudioLang,
                        AudioCh:     s.AudioCh,
                        SubLang:     s.SubLang,
                        SubCodec:    s.SubCodec,
                        TransVideoFrom: s.TransVideoFrom,
                        TransVideoTo:   s.TransVideoTo,
                        TransAudioFrom: s.TransAudioFrom,
                        TransAudioTo:   s.TransAudioTo,
                        VideoMethod: s.VideoMethod,
                        AudioMethod: s.AudioMethod,
                        TransReason: reasonText(s.VideoMethod, s.AudioMethod, s.TransReasons),
                        TransPct:    s.TransCompletion,
                        StreamPath:  streamPathLabel(s.TransContainer),
                        StreamDetail: func() string {
                            if !strings.EqualFold(s.PlayMethod, "Transcode") { return "" }
                            // Best-effort bitrate view
                            return fmt.Sprintf("%s (%s)", streamPathLabel(s.TransContainer), mbps(s.Bitrate))
                        }(),
                        IsPaused:    s.IsPaused,
                        ServerID:    "default-emby",
                        ServerType:  "emby",
                    })
                }
                return c.JSON(out)
            }
        }
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
            subsText = strconv.Itoa(s.SubtitleCount)
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
            TransPct: func() float64 {
                if s.TranscodeProgress > 0 { return s.TranscodeProgress }
                if s.DurationMs > 0 { return (float64(s.PositionMs) / float64(s.DurationMs)) * 100 }
                return 0
            }(),
            IsPaused:       s.IsPaused,
        }
        // Streaming path and detail when transcoding
        if strings.EqualFold(s.PlayMethod, "Transcode") {
            entry.StreamPath = streamPathLabel(s.TranscodeContainer)
            entry.StreamDetail = fmt.Sprintf("%s (%s)", entry.StreamPath, mbps(s.Bitrate))
            entry.TransVideoBitrate = s.TranscodeBitrate
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
