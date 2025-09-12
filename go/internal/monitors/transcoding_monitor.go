package monitors

import (
    "database/sql"
    "fmt"
    "regexp"
    "strings"
    "sync"
    "time"

    "emby-analytics/internal/emby"
    "emby-analytics/internal/handlers/settings"
    "emby-analytics/internal/logging"
)

// TranscodingMonitor monitors active sessions and stops 4K video transcoding when enabled
type TranscodingMonitor struct {
	db       *sql.DB
	emby     *emby.Client
	quit     chan struct{}
	wg       sync.WaitGroup
	interval time.Duration
}

// NewTranscodingMonitor creates a new transcoding monitor
func NewTranscodingMonitor(db *sql.DB, embyClient *emby.Client, interval time.Duration) *TranscodingMonitor {
	if interval <= 0 {
		interval = 30 * time.Second // Default 30 seconds
	}

	return &TranscodingMonitor{
		db:       db,
		emby:     embyClient,
		quit:     make(chan struct{}),
		interval: interval,
	}
}

// Start begins monitoring for 4K video transcoding
func (tm *TranscodingMonitor) Start() {
	tm.wg.Add(1)
	go tm.monitorLoop()
	logging.Info("4K video transcoding monitor started", "interval", tm.interval)
}

// Stop gracefully stops the monitor
func (tm *TranscodingMonitor) Stop() {
	close(tm.quit)
	tm.wg.Wait()
	logging.Info("4K video transcoding monitor stopped")
}

// monitorLoop is the main monitoring loop
func (tm *TranscodingMonitor) monitorLoop() {
	defer tm.wg.Done()
	
	ticker := time.NewTicker(tm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.quit:
			return
		case <-ticker.C:
			if tm.isMonitoringEnabled() {
				tm.checkAndStopTranscodingSessions()
			}
		}
	}
}

// isMonitoringEnabled checks if the setting is enabled
func (tm *TranscodingMonitor) isMonitoringEnabled() bool {
	return settings.GetSettingBool(tm.db, "prevent_4k_video_transcoding", false)
}

// checkAndStopTranscodingSessions checks active sessions and stops 4K video transcoding
func (tm *TranscodingMonitor) checkAndStopTranscodingSessions() {
	sessions, err := tm.emby.GetActiveSessions()
	if err != nil {
		logging.Debug("Failed to get active sessions for transcoding monitor", "error", err)
		return
	}

	for _, session := range sessions {
        if tm.shouldStopSession(session) {
            logging.Info("Stopping 4K video transcoding session", 
                "session_id", session.SessionID,
                "user", session.UserName,
                "item", session.ItemName,
                "device", session.Device)

            // Try to notify the user on the client before stopping playback
            // so it doesn't feel like an unexplained interruption.
            header := "4K Transcoding Blocked"
            body := fmt.Sprintf("This server blocks 4K video transcoding. Item: %s. Try a lower quality or direct play.", strings.TrimSpace(session.ItemName))
            if err := tm.emby.SendMessage(session.SessionID, header, body, 5000); err != nil {
                logging.Debug("Failed to send session message before stop", "error", err, "session_id", session.SessionID)
            } else {
                // Small delay to give the client a chance to render the message
                time.Sleep(750 * time.Millisecond)
            }

            if err := tm.emby.Stop(session.SessionID); err != nil {
                logging.Error("Failed to stop 4K video transcoding session", 
                    "error", err,
                    "session_id", session.SessionID,
                    "user", session.UserName)
            } else {
                logging.Info("Successfully stopped 4K video transcoding session", 
                    "session_id", session.SessionID,
                    "user", session.UserName,
                    "item", session.ItemName)
            }
        }
    }
}

// shouldStopSession determines if a session should be stopped based on 4K video transcoding
func (tm *TranscodingMonitor) shouldStopSession(session emby.EmbySession) bool {
	// Check if there's a playing item
	if session.ItemID == "" {
		return false
	}

	// Check if this is 4K content
	if !tm.is4KContent(session) {
		return false
	}

	// Check if video is being transcoded (this is the key check)
	if !tm.isVideoTranscoding(session) {
		return false
	}

	return true
}

// is4KContent determines if the content being played is 4K resolution
func (tm *TranscodingMonitor) is4KContent(session emby.EmbySession) bool {
	// Check width directly from session (4K is typically 1921-3840 pixels wide)
	if session.Width >= 1921 && session.Width <= 3840 {
		return true
	}

	// Fallback: check item name for 4K indicators
	if session.ItemName != "" {
		displayTitle := strings.ToLower(session.ItemName)
		if tm.contains4KMarker(displayTitle) {
			return true
		}
	}

	return false
}

// contains4KMarker checks if a string contains 4K resolution markers
func (tm *TranscodingMonitor) contains4KMarker(text string) bool {
	// Regex to match 4K resolution indicators
	pattern := regexp.MustCompile(`\b(4k|2160p)\b`)
	return pattern.MatchString(text)
}

// isVideoTranscoding determines if video is being transcoded in the session
func (tm *TranscodingMonitor) isVideoTranscoding(session emby.EmbySession) bool {
    // Check VideoMethod for direct video transcoding indication
    if strings.ToLower(session.VideoMethod) == "transcode" {
        return true
    }

    // Check if video codec is being converted (transcoded)
    if session.TransVideoFrom != "" && session.TransVideoTo != "" {
        // If source and target codecs are different, video is being transcoded
        if strings.ToLower(session.TransVideoFrom) != strings.ToLower(session.TransVideoTo) {
            return true
        }
    }

    // Look at explicit transcode reasons for video-related causes
    if len(session.TransReasons) > 0 {
        reasons := strings.ToLower(strings.Join(session.TransReasons, ","))
        videoIndicators := []string{
            "videocodecnotsupported", "video codec not supported",
            "videoprofilenotsupported", "video profile not supported",
            "videolevelnotsupported", "video level not supported",
            "videoframeratenotsupported", "video framerate not supported",
            "videobitratenotsupported", "video bitrate not supported",
            "videoresolutionnotsupported", "video resolution not supported",
        }
        for _, ind := range videoIndicators {
            if strings.Contains(reasons, ind) {
                return true
            }
        }
    }

    return false
}
