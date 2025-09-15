package media

import (
    "time"
    "strings"

    emby "emby-analytics/internal/emby"
)

// EmbyAdapter implements MediaServerClient by wrapping the existing Emby client
type EmbyAdapter struct {
    cfg ServerConfig
    c   *emby.Client
}

// NewEmbyAdapter constructs an EmbyAdapter from a ServerConfig
func NewEmbyAdapter(cfg ServerConfig) *EmbyAdapter {
    cli := emby.New(cfg.BaseURL, cfg.APIKey)
    return &EmbyAdapter{cfg: cfg, c: cli}
}

// Identification
func (e *EmbyAdapter) GetServerID() string   { return e.cfg.ID }
func (e *EmbyAdapter) GetServerType() ServerType { return ServerTypeEmby }
func (e *EmbyAdapter) GetServerName() string { return e.cfg.Name }

// Core
func (e *EmbyAdapter) GetActiveSessions() ([]Session, error) {
    emSessions, err := e.c.GetActiveSessions()
    if err != nil {
        return nil, err
    }
    out := make([]Session, 0, len(emSessions))
    for _, s := range emSessions {
        out = append(out, e.convertSession(s))
    }
    return out, nil
}

func (e *EmbyAdapter) GetSystemInfo() (*SystemInfo, error) {
    info, err := e.c.GetSystemInfo()
    if err != nil {
        return nil, err
    }
    return &SystemInfo{ID: info.ID, Name: info.Name, ServerType: ServerTypeEmby}, nil
}

func (e *EmbyAdapter) GetUsers() ([]User, error) {
    users, err := e.c.GetUsers()
    if err != nil { return nil, err }
    out := make([]User, 0, len(users))
    for _, u := range users {
        out = append(out, User{ID: u.Id, Name: u.Name, ServerID: e.cfg.ID, ServerType: ServerTypeEmby})
    }
    return out, nil
}

// Items
func (e *EmbyAdapter) ItemsByIDs(ids []string) ([]MediaItem, error) {
    items, err := e.c.ItemsByIDs(ids)
    if err != nil { return nil, err }
    out := make([]MediaItem, 0, len(items))
    for _, it := range items {
        mi := MediaItem{
            ID:         it.Id,
            ServerID:   e.cfg.ID,
            ServerType: ServerTypeEmby,
            Name:       it.Name,
            Type:       it.Type,
            SeriesID:   it.SeriesId,
            SeriesName: it.SeriesName,
            ParentIndexNumber: it.ParentIndexNumber,
            IndexNumber:       it.IndexNumber,
            ProductionYear:    it.ProductionYear,
        }
        out = append(out, mi)
    }
    return out, nil
}

func (e *EmbyAdapter) GetUserPlayHistory(userID string, daysBack int) ([]PlayHistoryItem, error) {
    items, err := e.c.GetUserPlayHistory(userID, daysBack)
    if err != nil { return nil, err }
    out := make([]PlayHistoryItem, 0, len(items))
    for _, it := range items {
        out = append(out, PlayHistoryItem{
            ID:          it.Id,
            ServerID:    e.cfg.ID,
            ServerType:  ServerTypeEmby,
            Name:        it.Name,
            Type:        it.Type,
            DatePlayed:  it.DatePlayed,
            PlaybackPos: it.PlaybackPos / 10_000, // ticks -> ms
            UserID:      it.UserID,
        })
    }
    return out, nil
}

// Controls
func (e *EmbyAdapter) PauseSession(sessionID string) error   { return e.c.Pause(sessionID) }
func (e *EmbyAdapter) UnpauseSession(sessionID string) error { return e.c.Unpause(sessionID) }
func (e *EmbyAdapter) StopSession(sessionID string) error    { return e.c.Stop(sessionID) }
func (e *EmbyAdapter) SendMessage(sessionID, header, text string, timeoutMs int) error {
    return e.c.SendMessage(sessionID, header, text, timeoutMs)
}

// Health
func (e *EmbyAdapter) CheckHealth() (*ServerHealth, error) {
    start := time.Now()
    _, err := e.c.GetSystemInfo()
    rt := time.Since(start).Milliseconds()
    h := &ServerHealth{ServerID: e.cfg.ID, ServerType: ServerTypeEmby, ServerName: e.cfg.Name, ResponseTime: rt, LastCheck: time.Now()}
    if err != nil { h.IsReachable = false; h.Error = err.Error(); return h, err }
    h.IsReachable = true
    return h, nil
}

// ---- helpers ----
func (e *EmbyAdapter) convertSession(s emby.EmbySession) Session {
    sess := Session{
        ServerID:   e.cfg.ID,
        ServerType: ServerTypeEmby,
        SessionID:  s.SessionID,
        UserID:     s.UserID,
        UserName:   s.UserName,
        ItemID:     s.ItemID,
        ItemName:   s.ItemName,
        ItemType:   s.ItemType,
        PositionMs: s.PosTicks / 10_000,
        DurationMs: s.DurationTicks / 10_000,
        ClientApp:  s.App,
        DeviceName: s.Device,
        RemoteAddress: s.RemoteAddress,
        PlayMethod: s.PlayMethod,
        VideoCodec: strings.ToUpper(s.VideoCodec),
        AudioCodec: strings.ToUpper(s.AudioCodec),
        Container:  strings.ToUpper(s.Container),
        Width:      s.Width,
        Height:     s.Height,
        Bitrate:    s.Bitrate,
        AudioLanguage: s.AudioLang,
        AudioChannels: s.AudioCh,
        AudioDefault:  s.AudioDefault,
        SubtitleLanguage: s.SubLang,
        SubtitleCodec:    s.SubCodec,
        SubtitleCount:    s.SubsCount,
        DolbyVision: s.DolbyVision,
        HDR10:       s.HDR10,
        TranscodeContainer:  strings.ToUpper(s.TransContainer),
        TranscodeVideoCodec: strings.ToUpper(s.TransVideoTo),
        TranscodeAudioCodec: strings.ToUpper(s.TransAudioTo),
        TranscodeReasons:    s.TransReasons,
        TranscodeProgress:   s.TransCompletion,
        TranscodeWidth:      s.TransWidth,
        TranscodeHeight:     s.TransHeight,
        TranscodeBitrate:    s.TransVideoBitrate,
        VideoMethod: s.VideoMethod,
        AudioMethod: s.AudioMethod,
        IsPaused:    s.IsPaused,
        LastUpdate:  time.Now(),
    }
    return sess
}

