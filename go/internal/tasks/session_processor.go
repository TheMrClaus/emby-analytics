package tasks

import (
    "database/sql"
    "log"
    "sync"
    "time"

    "emby-analytics/internal/media"
    "emby-analytics/internal/logging"
    "strings"
)

// SessionProcessor implements the hybrid state-polling approach used by playback_reporting plugin
type SessionProcessor struct {
    DB              *sql.DB
    MultiServerMgr  *media.MultiServerManager
    trackedSessions map[string]*TrackedSession // Internal "live list"
    mu              sync.Mutex
    Intervalizer    *Intervalizer
}

// TrackedSession represents a session we're tracking internally
type TrackedSession struct {
    SessionFK  int64
    SessionID  string
    ServerID   string
    ServerType media.ServerType
    UserID     string
    ItemID     string
    StartTime  time.Time
    LastUpdate time.Time
    LastPosTicks int64
    AccumulatedSec int // sum of active (unpaused, progressing) seconds
    LastPaused    bool
    // CurrentIntervalID tracks the play_intervals.id for the active contiguous segment
    // so we don't overwrite previous segments when a session is re-activated later.
    CurrentIntervalID int64
}

// NewSessionProcessor creates a new session processor
func NewSessionProcessor(db *sql.DB, multiServerMgr *media.MultiServerManager) *SessionProcessor {
	return &SessionProcessor{
		DB:              db,
		MultiServerMgr:  multiServerMgr,
		trackedSessions: make(map[string]*TrackedSession),
		Intervalizer: &Intervalizer{
			DB: db,
			NoProgressTimeout: 15 * time.Minute,
			PausedTimeout: 24 * time.Hour, // Default to 24 hours for paused sessions
			SeekThreshold: 2 * time.Minute,
		},
	}
}

// ProcessActiveSessions implements the core algorithm from playback_reporting plugin
func (sp *SessionProcessor) ProcessActiveSessions() {
    // Get sessions from all enabled servers
    var activeSessions []media.Session
    if sp.MultiServerMgr != nil {
        sessions, err := sp.MultiServerMgr.GetAllSessions()
        if err != nil {
            logging.Error("Failed to get sessions from multi-server manager", "error", err)
            return
        }
        activeSessions = sessions
    }
    sp.mu.Lock()
    defer sp.mu.Unlock()

	logging.Debug("Session processor running", "active_sessions", len(activeSessions), "tracked_sessions", len(sp.trackedSessions))

	currentTime := time.Now().UTC()
	activeSessionMap := make(map[string]bool)

	// Step B: Process Active Sessions
    for _, session := range activeSessions {
        // Composite key to avoid collisions across servers
        sessionKey := session.ServerID + "|" + session.SessionID
        activeSessionMap[sessionKey] = true

        // Skip Live TV completely
        switch strings.ToLower(strings.TrimSpace(session.ItemType)) {
        case "tvchannel", "livetv", "channel", "tvprogram":
            continue
        }

        if tracked, exists := sp.trackedSessions[sessionKey]; exists {
            // Detect item change within the same session
            if tracked.ItemID != session.ItemID {
                log.Printf("[session-processor] Item changed within session %s: %s -> %s; rotating session row",
                    sessionKey, tracked.ItemID, session.ItemID)
                // Finalize previous item session
                sp.finalizeSession(tracked, currentTime)
                delete(sp.trackedSessions, sessionKey)
                // Start new session for the new item
                sp.startNewSession(session, currentTime)
                continue
            }
            // Same item: accumulate only when playing (not paused) and position advanced
            advancedSec := 0
            if !session.IsPaused {
                // Prefer player position delta when available
                curTicks := msToTicks(session.PositionMs)
                if curTicks > 0 && tracked.LastPosTicks > 0 {
                    deltaTicks := curTicks - tracked.LastPosTicks
                    if deltaTicks < 0 { deltaTicks = 0 }
                    advancedSec = int(deltaTicks / 10_000_000)
                }
                // Fallback: if position missing but not paused, approximate using wall time since last update
                if advancedSec == 0 && !tracked.LastUpdate.IsZero() {
                    advancedSec = int(currentTime.Sub(tracked.LastUpdate).Seconds())
                    if advancedSec < 0 { advancedSec = 0 }
                }
            }
            tracked.AccumulatedSec += advancedSec
            tracked.LastUpdate = currentTime
            tracked.LastPosTicks = msToTicks(session.PositionMs)
            tracked.LastPaused = session.IsPaused

            // Persist: end_ts reflects last seen; duration_seconds is accumulated active seconds
            sp.updateSessionDuration(tracked, currentTime)
        } else {
            // New session - add to tracked list and create database entry
            log.Printf("[session-processor] New session detected: %s (server:%s user:%s item:%s)", sessionKey, session.ServerID, session.UserID, session.ItemName)
            sp.startNewSession(session, currentTime)
        }
    }

	// Step C: Find What's Missing (The Crucial Part)
    for sessionKey, tracked := range sp.trackedSessions {
        if !activeSessionMap[sessionKey] {
            // Session has stopped - perform final update and remove from tracked list
            log.Printf("[session-processor] Session stopped: %s (user: %s)", sessionKey, tracked.UserID)
            sp.finalizeSession(tracked, currentTime)
            delete(sp.trackedSessions, sessionKey)
        }
    }
}

// startNewSession creates a new session in the database and adds it to tracked sessions
func (sp *SessionProcessor) startNewSession(session media.Session, startTime time.Time) {
    // Create play_session record
    sessionFK, err := sp.createPlaySession(session, startTime)
    if err != nil {
        log.Printf("[session-processor] Failed to create play session: %v", err)
        return
    }

    // Add to tracked sessions
    key := session.ServerID + "|" + session.SessionID
    sp.trackedSessions[key] = &TrackedSession{
        SessionFK:  sessionFK,
        SessionID:  session.SessionID,
        ServerID:   session.ServerID,
        ServerType: session.ServerType,
        UserID:     session.UserID,
        ItemID:     session.ItemID,
        StartTime:  startTime,
        LastUpdate: startTime,
        LastPosTicks: msToTicks(session.PositionMs),
        AccumulatedSec: 0,
        LastPaused: session.IsPaused,
        CurrentIntervalID: 0,
    }

    log.Printf("[session-processor] Started tracking session %s (FK: %d)", session.SessionID, sessionFK)
}

// updateSessionDuration updates the session duration in the database
func (sp *SessionProcessor) updateSessionDuration(tracked *TrackedSession, currentTime time.Time) {
    duration := tracked.AccumulatedSec

    _, err := sp.DB.Exec(`
        UPDATE play_sessions 
        SET ended_at = ?, is_active = true 
        WHERE id = ?
    `, currentTime.Unix(), tracked.SessionFK)

	if err != nil {
		log.Printf("[session-processor] Failed to update session duration: %v", err)
		return
	}

	// Create/update play interval
    sp.createOrUpdateInterval(tracked, currentTime, duration)
}

// finalizeSession performs final database updates when a session ends
func (sp *SessionProcessor) finalizeSession(tracked *TrackedSession, endTime time.Time) {
    duration := tracked.AccumulatedSec

	// Update play_session as ended
	_, err := sp.DB.Exec(`
		UPDATE play_sessions 
		SET ended_at = ?, is_active = false 
		WHERE id = ?
	`, endTime.Unix(), tracked.SessionFK)

	if err != nil {
		log.Printf("[session-processor] Failed to finalize session: %v", err)
		return
	}

	// Create final play interval
	sp.createOrUpdateInterval(tracked, endTime, duration)

	log.Printf("[session-processor] Finalized session %s (total duration: %d seconds)", tracked.SessionID, duration)
}

// createOrUpdateInterval creates or updates a play interval
func (sp *SessionProcessor) createOrUpdateInterval(tracked *TrackedSession, endTime time.Time, duration int) {
    if duration < 1 {
        return // Skip very short intervals
    }

    // Maintain multiple intervals per session (one per contiguous active segment):
    // - If we have a current interval for this tracked segment, update it
    // - Otherwise, insert a new interval and remember its id
    if tracked.CurrentIntervalID != 0 {
        _, uerr := sp.DB.Exec(`
            UPDATE play_intervals
            SET end_ts = ?, duration_seconds = ?
            WHERE id = ?
        `, endTime.Unix(), duration, tracked.CurrentIntervalID)
        if uerr != nil {
            log.Printf("[session-processor] Failed to update interval: %v", uerr)
        }
        return
    }

    res, ierr := sp.DB.Exec(`
        INSERT INTO play_intervals 
        (session_fk, item_id, user_id, start_ts, end_ts, start_pos_ticks, end_pos_ticks, duration_seconds, seeked)
        SELECT id, item_id, user_id, ?, ?, 0, 0, ?, 0
        FROM play_sessions
        WHERE id = ?
    `, tracked.StartTime.Unix(), endTime.Unix(), duration, tracked.SessionFK)
    if ierr != nil {
        log.Printf("[session-processor] Failed to insert interval: %v", ierr)
        return
    }
    newID, _ := res.LastInsertId()
    tracked.CurrentIntervalID = newID
}

// createPlaySession creates a new play_session record in the database
func (sp *SessionProcessor) createPlaySession(session media.Session, startTime time.Time) (int64, error) {
    // Check if a session already exists for this (server_id, session_id, item_id)
    var existingID int64
    err := sp.DB.QueryRow(`SELECT id FROM play_sessions WHERE server_id=? AND session_id=? AND item_id=?`, session.ServerID, session.SessionID, session.ItemID).Scan(&existingID)
    if err == nil {
        // Reactivate existing row and refresh transcode details (best effort)
        transcodeReasons := strings.Join(session.TranscodeReasons, ",")
        // Derive codec from/to
        videoFrom := strings.ToUpper(session.VideoCodec)
        videoTo := strings.ToUpper(session.TranscodeVideoCodec)
        audioFrom := strings.ToUpper(session.AudioCodec)
        audioTo := strings.ToUpper(session.TranscodeAudioCodec)
        _, _ = sp.DB.Exec(`
            UPDATE play_sessions 
            SET is_active = true, ended_at = NULL,
                play_method = ?,
                transcode_reasons = COALESCE(NULLIF(?, ''), transcode_reasons),
                video_method = COALESCE(NULLIF(?, ''), video_method),
                audio_method = COALESCE(NULLIF(?, ''), audio_method),
                video_codec_from = COALESCE(NULLIF(?, ''), video_codec_from),
                video_codec_to   = COALESCE(NULLIF(?, ''), video_codec_to),
                audio_codec_from = COALESCE(NULLIF(?, ''), audio_codec_from),
                audio_codec_to   = COALESCE(NULLIF(?, ''), audio_codec_to)
            WHERE id = ?
        `, session.PlayMethod, transcodeReasons, session.VideoMethod, session.AudioMethod,
            videoFrom, videoTo, audioFrom, audioTo, existingID)
        return existingID, nil
    }

    transcodeReasons := strings.Join(session.TranscodeReasons, ",")
    videoFrom := strings.ToUpper(session.VideoCodec)
    videoTo := strings.ToUpper(session.TranscodeVideoCodec)
    audioFrom := strings.ToUpper(session.AudioCodec)
    audioTo := strings.ToUpper(session.TranscodeAudioCodec)
    res, ierr := sp.DB.Exec(`
        INSERT INTO play_sessions
        (user_id, session_id, device_id, client_name, item_id, item_name, item_type,
         play_method, started_at, is_active, transcode_reasons, remote_address,
         video_method, audio_method, video_codec_from, video_codec_to,
         audio_codec_from, audio_codec_to, server_id, server_type)
        VALUES(?,?,?,?,?,?,?,?,?,true,?,?,?,?, ?, ?, ?, ?, ?, ?)
    `, session.UserID, session.SessionID, session.DeviceName, session.ClientApp,
        session.ItemID, session.ItemName, session.ItemType, session.PlayMethod,
        startTime.Unix(), transcodeReasons, session.RemoteAddress,
        session.VideoMethod, session.AudioMethod, videoFrom, videoTo, audioFrom, audioTo,
        session.ServerID, string(session.ServerType))

    if ierr != nil {
        return 0, ierr
    }

    return res.LastInsertId()
}

// msToTicks converts milliseconds to 100-nanosecond ticks
func msToTicks(ms int64) int64 {
    if ms <= 0 {
        return 0
    }
    return ms * 10_000
}
