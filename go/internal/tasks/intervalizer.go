package tasks

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"emby-analytics/internal/emby"
)

// Intervalizer and liveState structs remain the same...
type Intervalizer struct {
	DB                *sql.DB
	NoProgressTimeout time.Duration
	SeekThreshold     time.Duration
}

type liveState struct {
	SessionFK        int64
	UserID           string // Add UserID to liveState
	ItemID           string // Add ItemID to liveState
	LastPosTicks     int64
	LastEventTS      time.Time
	IsIntervalOpen   bool
	IntervalStartTS  time.Time
	IntervalStartPos int64
}

// CORRECTED: Capitalize to make them public, so handlers can access them.
var (
	LiveSessions = make(map[string]*liveState)
	LiveMutex    = &sync.Mutex{}
)

// NEW FUNCTION: Provides safe, concurrent access to live watch times.
// Returns a map of UserID -> seconds watched in the current interval.
func GetLiveUserWatchTimes() map[string]float64 {
	LiveMutex.Lock()
	defer LiveMutex.Unlock()

	watchTimes := make(map[string]float64)
	now := time.Now()

	for _, session := range LiveSessions {
		if session.IsIntervalOpen {
			// Calculate the duration of the current, unclosed interval
			duration := now.Sub(session.IntervalStartTS).Seconds()
			watchTimes[session.UserID] += duration
		}
	}
	return watchTimes
}

// NEW FUNCTION: Same as above, but for items.
// Returns a map of ItemID -> seconds watched in the current interval.
func GetLiveItemWatchTimes() map[string]float64 {
	LiveMutex.Lock()
	defer LiveMutex.Unlock()

	watchTimes := make(map[string]float64)
	now := time.Now()

	for _, session := range LiveSessions {
		if session.IsIntervalOpen {
			duration := now.Sub(session.IntervalStartTS).Seconds()
			watchTimes[session.ItemID] += duration
		}
	}
	return watchTimes
}

func sessionKey(sessionID, itemID string) string { return sessionID + "|" + itemID }

// Handle now uses the public (capitalized) lock and map
func (iz *Intervalizer) Handle(evt emby.EmbyEvent) {
	LiveMutex.Lock()
	defer LiveMutex.Unlock()
	// ... (rest of Handle function is unchanged) ...
	var data emby.PlaybackProgressData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		log.Printf("[intervalizer] failed to unmarshal event data for %s: %v", evt.MessageType, err)
		return
	}

	if data.NowPlaying.ID == "" {
		return
	}

	switch evt.MessageType {
	case "PlaybackStart":
		iz.onStart(data)
	case "PlaybackProgress":
		iz.onProgress(data)
	case "PlaybackStopped":
		iz.onStop(data)
	}
}

func (iz *Intervalizer) onStart(d emby.PlaybackProgressData) {
	k := sessionKey(d.SessionID, d.NowPlaying.ID)
	now := time.Now().UTC()

	sessionFK, err := upsertSession(iz.DB, d)
	if err != nil {
		log.Printf("[intervalizer] onStart upsertSession failed: %v", err)
		return
	}

	insertEvent(iz.DB, sessionFK, "start", d.PlayState.IsPaused, d.PlayState.PositionTicks)

	s := &liveState{
		SessionFK:      sessionFK,
		UserID:         d.UserID,        // Store UserID
		ItemID:         d.NowPlaying.ID, // Store ItemID
		LastPosTicks:   d.PlayState.PositionTicks,
		LastEventTS:    now,
		IsIntervalOpen: false,
	}
	LiveSessions[k] = s // Use public map
}

func (iz *Intervalizer) onProgress(d emby.PlaybackProgressData) {
	k := sessionKey(d.SessionID, d.NowPlaying.ID)
	s, ok := LiveSessions[k] // Use public map
	if !ok {
		iz.onStart(d)
		s, ok = LiveSessions[k] // Use public map
		if !ok {
			return
		}
	}
	// ... (rest of onProgress is unchanged) ...
	now := time.Now().UTC()
	insertEvent(iz.DB, s.SessionFK, "progress", d.PlayState.IsPaused, d.PlayState.PositionTicks)

	if d.PlayState.IsPaused {
		if s.IsIntervalOpen {
			iz.closeInterval(s, now, d.PlayState.PositionTicks, false)
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
			iz.closeInterval(s, now, s.LastPosTicks, true)
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
	s, ok := LiveSessions[k] // Use public map
	if !ok {
		return
	}
	now := time.Now().UTC()

	insertEvent(iz.DB, s.SessionFK, "stop", false, d.PlayState.PositionTicks)
	if s.IsIntervalOpen {
		iz.closeInterval(s, now, d.PlayState.PositionTicks, false)
	}

	_, _ = iz.DB.Exec(`UPDATE play_sessions SET ended_at = ?, is_active = false WHERE id = ?`, now.Unix(), s.SessionFK)
	delete(LiveSessions, k) // Use public map
}

// TickTimeoutSweep now uses the public (capitalized) lock and map
func (iz *Intervalizer) TickTimeoutSweep() {
	LiveMutex.Lock()
	defer LiveMutex.Unlock()

	now := time.Now().UTC()
	for k, s := range LiveSessions {
		if now.Sub(s.LastEventTS) >= iz.NoProgressTimeout {
			log.Printf("[intervalizer] timing out session %s", k)
			if s.IsIntervalOpen {
				iz.closeInterval(s, s.LastEventTS, s.LastPosTicks, false)
			}
			_, _ = iz.DB.Exec(`UPDATE play_sessions SET ended_at = ?, is_active = false WHERE id = ?`, s.LastEventTS.Unix(), s.SessionFK)
			delete(LiveSessions, k)
		}
	}
}

// ... (rest of the file is unchanged) ...
func (iz *Intervalizer) closeInterval(s *liveState, end time.Time, endPos int64, seeked bool) {
	start := s.IntervalStartTS
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
	`, start.Unix(), end.Unix(), s.IntervalStartPos, endPos, dur, boolToInt(seeked), s.SessionFK)

	if err != nil {
		log.Printf("[intervalizer] failed to insert interval: %v", err)
	}
	s.IsIntervalOpen = false
}

func upsertSession(db *sql.DB, d emby.PlaybackProgressData) (int64, error) {
	var id int64
	err := db.QueryRow(`SELECT id FROM play_sessions WHERE session_id=? AND item_id=? AND is_active = true`, d.SessionID, d.NowPlaying.ID).Scan(&id)
	if err == nil {
		return id, nil
	}

	now := time.Now().UTC().Unix()
	res, err := db.Exec(`
		INSERT INTO play_sessions(user_id, session_id, device_id, client_name, item_id, item_name, item_type, play_method, started_at, is_active)
		VALUES(?,?,?,?,?,?,?,?,?,true)
	`, d.UserID, d.SessionID, d.DeviceID, d.Client, d.NowPlaying.ID, d.NowPlaying.Name, d.NowPlaying.Type, d.PlayMethod, now)

	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertEvent(db *sql.DB, fk int64, kind string, paused bool, pos int64) {
	_, err := db.Exec(`
		INSERT INTO play_events(session_fk, kind, is_paused, position_ticks, created_at)
		VALUES(?,?,?,?,?)
	`, fk, kind, boolToInt(paused), pos, time.Now().UTC().Unix())
	if err != nil {
		log.Printf("[intervalizer] failed to insert event: %v", err)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
