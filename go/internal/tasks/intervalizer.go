package tasks

import (
	"database/sql"
	"emby-analytics/internal/logging"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"emby-analytics/internal/emby"
)

type Intervalizer struct {
	DB                *sql.DB
	NoProgressTimeout time.Duration
	SeekThreshold     time.Duration
}

type liveState struct {
    SessionFK        int64
    SessionID        string // Session identifier from Emby
    DeviceID         string // Device identifier from Emby
    UserID           string
    ItemID           string
    ItemType         string
    LastPosTicks     int64
    LastEventTS      time.Time
    SessionStartTS   time.Time
    IsIntervalOpen   bool
    IntervalStartTS  time.Time
	IntervalStartPos int64
	// Tracks whether we have recorded any interval for this session
	HadAnyInterval bool
}

var (
    LiveSessions = make(map[string]*liveState)
    LiveMutex    = &sync.Mutex{}
)

// NEW: Expose live watch times for active intervals
func isLiveTVType(t string) bool {
    switch strings.ToLower(strings.TrimSpace(t)) {
    case "tvchannel", "livetv", "channel", "tvprogram":
        return true
    default:
        return false
    }
}

// NEW: Expose live watch times for active intervals (excluding Live TV)
func GetLiveUserWatchTimes() map[string]float64 {
    LiveMutex.Lock()
    defer LiveMutex.Unlock()
    watchTimes := make(map[string]float64)
    now := time.Now()
    for _, session := range LiveSessions {
        if session.IsIntervalOpen {
            duration := now.Sub(session.IntervalStartTS).Seconds()
            watchTimes[session.UserID] += duration
        }
    }
    return watchTimes
}

// Excludes Live TV
func GetLiveItemWatchTimes() map[string]float64 {
    LiveMutex.Lock()
    defer LiveMutex.Unlock()
    watchTimes := make(map[string]float64)
    now := time.Now()
    for _, session := range LiveSessions {
        if session.IsIntervalOpen && !isLiveTVType(session.ItemType) {
            duration := now.Sub(session.IntervalStartTS).Seconds()
            watchTimes[session.ItemID] += duration
        }
    }
    return watchTimes
}

// Helper specifically for TopUsers to exclude Live TV
func GetLiveUserWatchTimesExcludingLiveTV() map[string]float64 {
    LiveMutex.Lock()
    defer LiveMutex.Unlock()
    watchTimes := make(map[string]float64)
    now := time.Now()
    for _, session := range LiveSessions {
        if session.IsIntervalOpen && !isLiveTVType(session.ItemType) {
            duration := now.Sub(session.IntervalStartTS).Seconds()
            watchTimes[session.UserID] += duration
        }
    }
    return watchTimes
}

func sessionKey(sessionID, itemID string) string { return sessionID + "|" + itemID }

func (iz *Intervalizer) Handle(evt emby.EmbyEvent) {
    logging.Debug("Received event: %s", evt.MessageType)

	LiveMutex.Lock()
	defer LiveMutex.Unlock()
	var data emby.PlaybackProgressData
    if err := json.Unmarshal(evt.Data, &data); err != nil {
        logging.Debug("JSON unmarshal error: %v", err)
        return
    }
    if data.NowPlaying.ID == "" {
        logging.Debug("[intervalizer] Empty NowPlaying.ID, skipping event")
        return
    }
    // Skip Live TV content entirely
    if isLiveTVType(data.NowPlaying.Type) {
        logging.Debug("[intervalizer] Skipping Live TV event for item %s", data.NowPlaying.ID)
        return
    }

	logging.Debug("Processing %s for user %s, item %s", evt.MessageType, data.UserID, data.NowPlaying.Name)

	switch evt.MessageType {
	case "PlaybackStart":
		iz.onStart(data)
	case "PlaybackProgress":
		iz.onProgress(data)
	case "PlaybackStopped":
		iz.onStop(data)
	default:
		logging.Debug("Unhandled event type: %s", evt.MessageType)
	}
}

func (iz *Intervalizer) onStart(d emby.PlaybackProgressData) {
	logging.Debug("onStart called for user %s, item %s, session %s", d.UserID, d.NowPlaying.Name, d.SessionID)

	k := sessionKey(d.SessionID, d.NowPlaying.ID)
	now := time.Now().UTC()
	sessionFK, err := upsertSession(iz.DB, d)
	if err != nil {
		logging.Debug("onStart upsertSession failed: %v", err)
		return
	}
	logging.Debug("onStart created session FK: %d", sessionFK)

	insertEvent(iz.DB, sessionFK, "start", d.PlayState.IsPaused, d.PlayState.PositionTicks)
    s := &liveState{
        SessionFK:      sessionFK,
        SessionID:      d.SessionID,
        DeviceID:       d.DeviceID,
        UserID:         d.UserID,
        ItemID:         d.NowPlaying.ID,
        ItemType:       d.NowPlaying.Type,
        LastPosTicks:   d.PlayState.PositionTicks,
        LastEventTS:    now,
        SessionStartTS: now, // Store the absolute start time
        IsIntervalOpen: false,
    }
	LiveSessions[k] = s
	logging.Debug("onStart complete, added to LiveSessions: %s", k)
}

func (iz *Intervalizer) onProgress(d emby.PlaybackProgressData) {
	k := sessionKey(d.SessionID, d.NowPlaying.ID)
	s, ok := LiveSessions[k]
	if !ok {
		iz.onStart(d)
		s, ok = LiveSessions[k]
		if !ok {
			return
		}
	}
	now := time.Now().UTC()
	insertEvent(iz.DB, s.SessionFK, "progress", d.PlayState.IsPaused, d.PlayState.PositionTicks)
	if d.PlayState.IsPaused {
		if s.IsIntervalOpen {
			iz.closeInterval(s, s.IntervalStartTS, now, s.IntervalStartPos, d.PlayState.PositionTicks, false)
		}
		s.LastEventTS = now
		s.LastPosTicks = d.PlayState.PositionTicks
		return
	}
	const ticksPerSecond = 10000000
	posJumpTicks := d.PlayState.PositionTicks - s.LastPosTicks
	if posJumpTicks < 0 {
		posJumpTicks = -posJumpTicks
	}
	if time.Duration(posJumpTicks/ticksPerSecond)*time.Second >= iz.SeekThreshold {
		if s.IsIntervalOpen {
			iz.closeInterval(s, s.IntervalStartTS, now, s.IntervalStartPos, s.LastPosTicks, true)
		}
		s.IsIntervalOpen = false
	}
	if !s.IsIntervalOpen {
		s.IsIntervalOpen = true
		s.IntervalStartTS = now
		s.IntervalStartPos = d.PlayState.PositionTicks
	}
	s.LastEventTS = now
	s.LastPosTicks = d.PlayState.PositionTicks
}

func (iz *Intervalizer) onStop(d emby.PlaybackProgressData) {
	k := sessionKey(d.SessionID, d.NowPlaying.ID)
	s, ok := LiveSessions[k]
	if !ok {
		return
	}
	now := time.Now().UTC()

	insertEvent(iz.DB, s.SessionFK, "stop", false, d.PlayState.PositionTicks)

	if s.IsIntervalOpen {
		// If an interval was open, close it normally.
		iz.closeInterval(s, s.IntervalStartTS, now, s.IntervalStartPos, d.PlayState.PositionTicks, false)
	} else if !s.HadAnyInterval && !s.SessionStartTS.IsZero() && s.LastPosTicks > 0 {
		// Create interval only for sessions that never produced any progress-based intervals
		iz.closeInterval(s, s.SessionStartTS, now, 0, d.PlayState.PositionTicks, false)
	}

	_, _ = iz.DB.Exec(`UPDATE play_sessions SET ended_at = ?, is_active = false WHERE id = ?`, now.Unix(), s.SessionFK)
	delete(LiveSessions, k)
}

// DetectStoppedSessions identifies sessions that are no longer active and creates PlaybackStopped events
func (iz *Intervalizer) DetectStoppedSessions(activeSessionKeys map[string]bool) {
	LiveMutex.Lock()
	defer LiveMutex.Unlock()

	logging.Debug("DetectStoppedSessions called with %d active sessions, %d live sessions", len(activeSessionKeys), len(LiveSessions))

	for liveSessionKey, liveSession := range LiveSessions {
		// Check if this session is still active by SessionID only
		// This avoids the Device vs DeviceID mismatch between WebSocket and REST API
		sessionFound := false
		for activeKey := range activeSessionKeys {
			if strings.HasPrefix(activeKey, liveSession.SessionID+"|") {
				sessionFound = true
				break
			}
		}

		// If this live session is not in the current active sessions, it has stopped
		if !sessionFound {
			logging.Debug("Detected stopped session: %s (user: %s)", liveSessionKey, liveSession.UserID)

			// Create synthetic PlaybackStopped event data
			stoppedData := emby.PlaybackProgressData{
				UserID:    liveSession.UserID,
				SessionID: liveSession.SessionID,
				DeviceID:  liveSession.DeviceID,
				NowPlaying: struct {
					ID           string `json:"Id"`
					RunTimeTicks int64  `json:"RunTimeTicks"`
					Type         string `json:"Type"`
					Name         string `json:"Name"`
				}{
					ID: liveSession.ItemID,
					// We don't have full item data, but ID is sufficient for session cleanup
				},
				PlayState: struct {
					IsPaused            bool    `json:"IsPaused"`
					PositionTicks       int64   `json:"PositionTicks"`
					PlaybackRate        float64 `json:"PlaybackRate"`
					AudioStreamIndex    *int    `json:"AudioStreamIndex"`
					SubtitleStreamIndex *int    `json:"SubtitleStreamIndex"`
				}{
					PositionTicks: liveSession.LastPosTicks,
				},
			}

			syntheticStopEvent := emby.EmbyEvent{
				MessageType: "PlaybackStopped",
			}

			data, err := json.Marshal(stoppedData)
			if err != nil {
				logging.Debug("Failed to marshal stopped data: %v", err)
				continue
			}
			syntheticStopEvent.Data = json.RawMessage(data)

			logging.Debug("Created synthetic PlaybackStopped for session %s", liveSessionKey)
			iz.Handle(syntheticStopEvent)
		}
	}
}

func (iz *Intervalizer) TickTimeoutSweep() {
	LiveMutex.Lock()
	defer LiveMutex.Unlock()
	now := time.Now().UTC()
	for k, s := range LiveSessions {
		if now.Sub(s.LastEventTS) >= iz.NoProgressTimeout {
			logging.Debug("timing out session %s", k)
			if s.IsIntervalOpen {
				iz.closeInterval(s, s.IntervalStartTS, s.LastEventTS, s.IntervalStartPos, s.LastPosTicks, false)
			}
			_, _ = iz.DB.Exec(`UPDATE play_sessions SET ended_at = ?, is_active = false WHERE id = ?`, s.LastEventTS.Unix(), s.SessionFK)
			delete(LiveSessions, k)
		}
	}
}

func (iz *Intervalizer) closeInterval(s *liveState, start time.Time, end time.Time, startPos int64, endPos int64, seeked bool) {
	if end.Before(start) || end.Sub(start).Seconds() < 1 {
		s.IsIntervalOpen = false
		return
	}
	dur := int(end.Sub(start).Seconds())
	_, err := iz.DB.Exec(`
        INSERT INTO play_intervals (session_fk, item_id, user_id, start_ts, end_ts, start_pos_ticks, end_pos_ticks, duration_seconds, seeked)
        SELECT id, item_id, user_id, ?, ?, ?, ?, ?, ?
        FROM play_sessions
        WHERE id = ?
    `, start.Unix(), end.Unix(), startPos, endPos, dur, boolToInt(seeked), s.SessionFK)
	if err != nil {
		logging.Debug("failed to insert interval: %v", err)
	}
	s.IsIntervalOpen = false
	s.HadAnyInterval = true
}

// ... (upsertSession, insertEvent, boolToInt are unchanged)
func upsertSession(db *sql.DB, d emby.PlaybackProgressData) (int64, error) {
	var id int64
	// Check for ANY existing session (active or inactive)
	err := db.QueryRow(`SELECT id FROM play_sessions WHERE session_id=? AND item_id=?`, d.SessionID, d.NowPlaying.ID).Scan(&id)
	if err == nil {
		// Found existing session, reactivate it

		// Convert TranscodeReasons slice to comma-separated string
		var transcodeReasonsStr string
		if len(d.TranscodeReasons) > 0 {
			transcodeReasonsStr = strings.Join(d.TranscodeReasons, ",")
		}

		// Determine detailed playback methods from available data
		videoMethod, audioMethod, videoCodecFrom, videoCodecTo, audioCodecFrom, audioCodecTo := determineDetailedMethods(d)

        _, updateErr := db.Exec(`
			UPDATE play_sessions 
			SET user_id=?, device_id=?, client_name=?, item_name=?, item_type=?, play_method=?, 
				ended_at=NULL, is_active=true, transcode_reasons=?, remote_address=?,
				video_method=?, audio_method=?, video_codec_from=?, video_codec_to=?, 
				audio_codec_from=?, audio_codec_to=?
			WHERE id=?
		`, d.UserID, d.DeviceID, d.Client, d.NowPlaying.Name, d.NowPlaying.Type, d.PlayMethod, transcodeReasonsStr, d.RemoteEndPoint, videoMethod, audioMethod, videoCodecFrom, videoCodecTo, audioCodecFrom, audioCodecTo, id)
		if updateErr != nil {
			return 0, updateErr
		}
		return id, nil
	}

	// No existing session found, create new one
	now := time.Now().UTC().Unix()

	// Convert TranscodeReasons slice to comma-separated string
	var transcodeReasonsStr string
	if len(d.TranscodeReasons) > 0 {
		transcodeReasonsStr = strings.Join(d.TranscodeReasons, ",")
	}

	// Determine detailed playback methods from available data
	videoMethod, audioMethod, videoCodecFrom, videoCodecTo, audioCodecFrom, audioCodecTo := determineDetailedMethods(d)

	res, err := db.Exec(`
		INSERT INTO play_sessions(user_id, session_id, device_id, client_name, item_id, item_name, item_type, play_method, started_at, is_active, transcode_reasons, remote_address, video_method, audio_method, video_codec_from, video_codec_to, audio_codec_from, audio_codec_to)
		VALUES(?,?,?,?,?,?,?,?,?,true,?,?,?,?,?,?,?,?)
	`, d.UserID, d.SessionID, d.DeviceID, d.Client, d.NowPlaying.ID, d.NowPlaying.Name, d.NowPlaying.Type, d.PlayMethod, now, transcodeReasonsStr, d.RemoteEndPoint, videoMethod, audioMethod, videoCodecFrom, videoCodecTo, audioCodecFrom, audioCodecTo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertEvent(db *sql.DB, fk int64, kind string, paused bool, pos int64) {
	_, err := db.Exec(`INSERT INTO play_events(session_fk, kind, is_paused, position_ticks, created_at) VALUES(?,?,?,?,?)`, fk, kind, boolToInt(paused), pos, time.Now().UTC().Unix())
	if err != nil {
		logging.Debug("failed to insert event: %v", err)
	}
}
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// determineDetailedMethods analyzes the available transcode data to determine video/audio methods
func determineDetailedMethods(d emby.PlaybackProgressData) (videoMethod, audioMethod, videoCodecFrom, videoCodecTo, audioCodecFrom, audioCodecTo string) {
	// Default to DirectPlay
	videoMethod = "DirectPlay"
	audioMethod = "DirectPlay"

	// If not transcoding at all, everything is direct
	if d.PlayMethod != "Transcode" || len(d.TranscodeReasons) == 0 {
		return
	}

	// Analyze transcode reasons to determine what's being transcoded
	reasonsStr := strings.ToLower(strings.Join(d.TranscodeReasons, " "))

	// Check for video transcoding indicators
	videoIndicators := []string{
		"videocodecnotsupported", "video codec not supported",
		"videoprofilenotsupported", "video profile not supported",
		"videolevelnotsupported", "video level not supported",
		"videoframeratenotsupported", "video framerate not supported",
		"videobitratenotsupported", "video bitrate not supported",
		"videoresolutionnotsupported", "video resolution not supported",
		"subtitlecodecnotsupported", "subtitle", "burn", // subtitle burn-in affects video
		"containernotcurrentsupported", // container issues often require video re-encode
	}

	// Check for audio transcoding indicators
	audioIndicators := []string{
		"audiocodecnotsupported", "audio codec not supported",
		"audioprofilenotsupported", "audio profile not supported",
		"audiobitratenotsupported", "audio bitrate not supported",
		"audiochannelsnotsupported", "audio channels not supported",
		"audiosampleratenotsupported", "audio sample rate not supported",
	}

	// Determine what's transcoding
	hasVideoTranscode := false
	hasAudioTranscode := false

	for _, indicator := range videoIndicators {
		if strings.Contains(reasonsStr, indicator) {
			hasVideoTranscode = true
			break
		}
	}

	for _, indicator := range audioIndicators {
		if strings.Contains(reasonsStr, indicator) {
			hasAudioTranscode = true
			break
		}
	}

	// If we found specific indicators, use them
	if hasVideoTranscode {
		videoMethod = "Transcode"
	}
	if hasAudioTranscode {
		audioMethod = "Transcode"
	}

	// If no specific indicators but we know it's transcoding, make best guess
	// Most transcode scenarios involve video, so default to video if unclear
	if !hasVideoTranscode && !hasAudioTranscode && d.PlayMethod == "Transcode" {
		videoMethod = "Transcode" // Default assumption
	}

	return
}
