package tasks

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"emby-analytics/internal/emby"
)

// SessionProcessor implements the hybrid state-polling approach used by playback_reporting plugin
type SessionProcessor struct {
	DB             *sql.DB
	trackedSessions map[string]*TrackedSession // Internal "live list" 
	mu             sync.Mutex
}

// TrackedSession represents a session we're tracking internally
type TrackedSession struct {
	SessionFK   int64
	SessionID   string
	UserID      string
	ItemID      string
	StartTime   time.Time
	LastUpdate  time.Time
}

// NewSessionProcessor creates a new session processor
func NewSessionProcessor(db *sql.DB) *SessionProcessor {
	return &SessionProcessor{
		DB:              db,
		trackedSessions: make(map[string]*TrackedSession),
	}
}

// ProcessActiveSessions implements the core algorithm from playback_reporting plugin
func (sp *SessionProcessor) ProcessActiveSessions(activeSessions []emby.EmbySession) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	log.Printf("[session-processor] Processing %d active sessions, %d tracked sessions", len(activeSessions), len(sp.trackedSessions))

	currentTime := time.Now().UTC()
	activeSessionMap := make(map[string]bool)

	// Step B: Process Active Sessions
	for _, session := range activeSessions {
		sessionKey := session.SessionID // Use SessionID as the key
		activeSessionMap[sessionKey] = true

		if tracked, exists := sp.trackedSessions[sessionKey]; exists {
			// Session already tracked - update duration in database
			log.Printf("[session-processor] Updating existing session %s", sessionKey)
			sp.updateSessionDuration(tracked, currentTime)
			tracked.LastUpdate = currentTime
		} else {
			// New session - add to tracked list and create database entry
			log.Printf("[session-processor] New session detected: %s (user: %s, item: %s)", sessionKey, session.UserID, session.ItemName)
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
func (sp *SessionProcessor) startNewSession(session emby.EmbySession, startTime time.Time) {
	// Create play_session record
	sessionFK, err := sp.createPlaySession(session, startTime)
	if err != nil {
		log.Printf("[session-processor] Failed to create play session: %v", err)
		return
	}

	// Add to tracked sessions
	sp.trackedSessions[session.SessionID] = &TrackedSession{
		SessionFK:  sessionFK,
		SessionID:  session.SessionID,
		UserID:     session.UserID,
		ItemID:     session.ItemID,
		StartTime:  startTime,
		LastUpdate: startTime,
	}

	log.Printf("[session-processor] Started tracking session %s (FK: %d)", session.SessionID, sessionFK)
}

// updateSessionDuration updates the session duration in the database
func (sp *SessionProcessor) updateSessionDuration(tracked *TrackedSession, currentTime time.Time) {
	duration := int(currentTime.Sub(tracked.StartTime).Seconds())
	
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
	duration := int(endTime.Sub(tracked.StartTime).Seconds())
	
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

	// For simplicity, create a single interval spanning the entire session
	// In a more sophisticated implementation, you'd track position changes
	_, err := sp.DB.Exec(`
		INSERT OR REPLACE INTO play_intervals 
		(session_fk, item_id, user_id, start_ts, end_ts, start_pos_ticks, end_pos_ticks, duration_seconds, seeked)
		SELECT id, item_id, user_id, ?, ?, 0, 0, ?, 0
		FROM play_sessions
		WHERE id = ?
	`, tracked.StartTime.Unix(), endTime.Unix(), duration, tracked.SessionFK)
	
	if err != nil {
		log.Printf("[session-processor] Failed to create interval: %v", err)
	}
}

// createPlaySession creates a new play_session record in the database
func (sp *SessionProcessor) createPlaySession(session emby.EmbySession, startTime time.Time) (int64, error) {
	res, err := sp.DB.Exec(`
		INSERT INTO play_sessions
		(user_id, session_id, device_id, client_name, item_id, item_name, item_type, 
		 play_method, started_at, is_active, transcode_reasons, remote_address,
		 video_method, audio_method, video_codec_from, video_codec_to, 
		 audio_codec_from, audio_codec_to)
		VALUES(?,?,?,?,?,?,?,?,?,true,'','','','','','','','')
	`, session.UserID, session.SessionID, session.Device, session.App, 
		session.ItemID, session.ItemName, session.ItemType, session.PlayMethod, 
		startTime.Unix())
		
	if err != nil {
		return 0, err
	}
	
	return res.LastInsertId()
}