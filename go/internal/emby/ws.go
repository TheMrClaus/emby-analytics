package emby

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type WSConfig struct {
	BaseURL string // e.g. http://emby:8096
	APIKey  string
}

type EmbyWS struct {
	Cfg                 WSConfig
	conn                *websocket.Conn
	cancel              context.CancelFunc
	Handler             func(evt EmbyEvent)
	StoppedSessionCheck func(activeSessionKeys map[string]bool) // NEW: callback for stopped session detection
}

type EmbyEvent struct {
	MessageType string          `json:"MessageType"`
	Data        json.RawMessage `json:"Data"`
}

// PlaybackProgressData is a trimmed down version of the event payload
type PlaybackProgressData struct {
	UserID           string   `json:"UserId"`
	SessionID        string   `json:"SessionId"`
	DeviceID         string   `json:"DeviceId"`
	Client           string   `json:"Client"`
	PlayMethod       string   `json:"PlayMethod"` // DirectPlay/DirectStream/Transcode
	RemoteEndPoint   string   `json:"RemoteEndPoint,omitempty"`
	TranscodeReasons []string `json:"TranscodeReasons,omitempty"`
	NowPlaying       struct {
		ID           string `json:"Id"`
		RunTimeTicks int64  `json:"RunTimeTicks"`
		Type         string `json:"Type"`
		Name         string `json:"Name"`
	} `json:"NowPlayingItem"`
	PlayState struct {
		IsPaused            bool    `json:"IsPaused"`
		PositionTicks       int64   `json:"PositionTicks"`
		PlaybackRate        float64 `json:"PlaybackRate"`
		AudioStreamIndex    *int    `json:"AudioStreamIndex"`    // Currently selected audio stream
		SubtitleStreamIndex *int    `json:"SubtitleStreamIndex"` // Currently selected subtitle stream
	} `json:"PlayState"`
}

// SessionData represents the structure of Sessions event data
type SessionData struct {
	UserID           string   `json:"UserId"`
	SessionID        string   `json:"SessionId,omitempty"`
	DeviceID         string   `json:"DeviceId,omitempty"`
	Client           string   `json:"Client,omitempty"`
	PlayMethod       string   `json:"PlayMethod,omitempty"` // Add this field
	RemoteEndPoint   string   `json:"RemoteEndPoint,omitempty"`
	TranscodeReasons []string `json:"TranscodeReasons,omitempty"`
	PlayState        struct {
		PositionTicks       int64   `json:"PositionTicks"`
		IsPaused            bool    `json:"IsPaused"`
		PlaybackRate        float64 `json:"PlaybackRate"`
		AudioStreamIndex    *int    `json:"AudioStreamIndex"`    // Currently selected audio stream
		SubtitleStreamIndex *int    `json:"SubtitleStreamIndex"` // Currently selected subtitle stream
	} `json:"PlayState"`
	NowPlayingItem *struct {
		ID           string `json:"Id"`
		Name         string `json:"Name"`
		Type         string `json:"Type"`
		RunTimeTicks int64  `json:"RunTimeTicks"`
	} `json:"NowPlayingItem"`
}

func (w *EmbyWS) dial() (*websocket.Conn, *http.Response, error) {
	u, err := url.Parse(w.Cfg.BaseURL)
	if err != nil {
		return nil, nil, err
	}
	scheme := "wss"
	if strings.HasPrefix(strings.ToLower(u.Scheme), "http") {
		if u.Scheme == "http" {
			scheme = "ws"
		}
	}
	// Emby socket endpoint is /embywebsocket
	u.Scheme = scheme
	u.Path = "/embywebsocket"
	q := u.Query()
	q.Set("api_key", w.Cfg.APIKey)
	q.Set("deviceId", "emby-analytics-go-client") // A unique identifier
	u.RawQuery = q.Encode()

	dialer := &websocket.Dialer{
		HandshakeTimeout:  15 * time.Second,
		EnableCompression: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // Allow self-signed certs
	}

	header := http.Header{
		"Accept": []string{"application/json"},
	}

	log.Printf("[emby-ws] Dialing %s", u.String())
	return dialer.Dial(u.String(), header)
}

func (w *EmbyWS) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	go func() {
		defer cancel()
		retry := 5 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, _, err := w.dial()
			if err != nil {
				log.Printf("[emby-ws] Dial failed: %v, retrying in %v", err, retry)
				time.Sleep(retry)
				continue
			}
			w.conn = conn

			log.Printf("[emby-ws] ✅ Connected")
			messageCount := 0
			conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			conn.SetPongHandler(func(appData string) error {
				conn.SetReadDeadline(time.Now().Add(90 * time.Second))
				return nil
			})

			// Send periodic pings
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
							log.Printf("[emby-ws] Ping failed: %v", err)
							return
						}
					}
				}
			}()

			for {
				select {
				case <-ctx.Done():
					conn.Close()
					return
				default:
				}

				var evt EmbyEvent
				if err := conn.ReadJSON(&evt); err != nil {
					log.Printf("[emby-ws] ReadJSON error: %v", err)
					break
				}

				messageCount++
				if evt.MessageType == "ForceKeepAlive" || evt.MessageType == "KeepAlive" {
					// Ignore keepalives
					continue
				}

				log.Printf("[emby-ws] 📨 %s (msg #%d)", evt.MessageType, messageCount)

				// Handle playback events
				if strings.HasPrefix(evt.MessageType, "Playback") {
					log.Printf("[emby-ws] ✅ PLAYBACK EVENT - Forwarding to handler")
					if w.Handler != nil {
						w.Handler(evt)
					}
				} else if evt.MessageType == "Sessions" {
					log.Printf("[emby-ws] ✅ SESSIONS EVENT - Converting to playback events")
					// Convert Sessions event to Playback events
					w.handleSessionsEvent(evt)
				} else {
					log.Printf("[emby-ws] ℹ️  Non-playback event (ignored): %s", evt.MessageType)
				}
			}
			// Reconnect on break
			log.Printf("[emby-ws] Connection lost after %d messages, reconnecting...", messageCount)
			time.Sleep(retry)
		}
	}()
}

// handleSessionsEvent converts Sessions events to Playback events
func (w *EmbyWS) handleSessionsEvent(evt EmbyEvent) {
	if w.Handler == nil {
		return
	}

	// Parse Sessions data
	var sessions []SessionData
	if err := json.Unmarshal(evt.Data, &sessions); err != nil {
		log.Printf("[emby-ws] Failed to parse Sessions data: %v", err)
		return
	}

	// Track which sessions are currently active
	activeSessionKeys := make(map[string]bool)

	for _, session := range sessions {
		sessionKey := session.SessionID + "|" + session.DeviceID // Use DeviceID as fallback identifier
		
		if session.NowPlayingItem != nil {
			// Active session - create PlaybackProgress event
			activeSessionKeys[sessionKey] = true
			
			// Convert to PlaybackProgressData format
			progressData := PlaybackProgressData{
				UserID:           session.UserID,
				SessionID:        session.SessionID,
				DeviceID:         session.DeviceID,
				Client:           session.Client,
				PlayMethod:       detectPlayMethod(session),
				RemoteEndPoint:   session.RemoteEndPoint,
				TranscodeReasons: session.TranscodeReasons,
				NowPlaying: struct {
					ID           string `json:"Id"`
					RunTimeTicks int64  `json:"RunTimeTicks"`
					Type         string `json:"Type"`
					Name         string `json:"Name"`
				}{
					ID:           session.NowPlayingItem.ID,
					RunTimeTicks: session.NowPlayingItem.RunTimeTicks,
					Type:         session.NowPlayingItem.Type,
					Name:         session.NowPlayingItem.Name,
				},
				PlayState: struct {
					IsPaused            bool    `json:"IsPaused"`
					PositionTicks       int64   `json:"PositionTicks"`
					PlaybackRate        float64 `json:"PlaybackRate"`
					AudioStreamIndex    *int    `json:"AudioStreamIndex"`
					SubtitleStreamIndex *int    `json:"SubtitleStreamIndex"`
				}{
					IsPaused:            session.PlayState.IsPaused,
					PositionTicks:       session.PlayState.PositionTicks,
					PlaybackRate:        session.PlayState.PlaybackRate,
					AudioStreamIndex:    session.PlayState.AudioStreamIndex,
					SubtitleStreamIndex: session.PlayState.SubtitleStreamIndex,
				},
			}

			// Create a synthetic PlaybackProgress event
			syntheticEvent := EmbyEvent{
				MessageType: "PlaybackProgress",
				Data:        nil, // Will be marshaled below
			}

			// Marshal the progress data
			data, err := json.Marshal(progressData)
			if err != nil {
				log.Printf("[emby-ws] Failed to marshal progress data: %v", err)
				continue
			}
			syntheticEvent.Data = json.RawMessage(data)

			log.Printf("[emby-ws] Created synthetic PlaybackProgress for user %s, item %s",
				progressData.UserID, progressData.NowPlaying.Name)

			// Send to intervalizer
			w.Handler(syntheticEvent)
		}
	}

	// NEW: Detect stopped sessions by comparing with live sessions
	// This is the critical fix for session completion detection
	if w.StoppedSessionCheck != nil {
		w.StoppedSessionCheck(activeSessionKeys)
	}
}

// detectPlayMethod applies the same logic as GetActiveSessions to determine PlayMethod
func detectPlayMethod(session SessionData) string {
	// Check if there are transcode reasons - strong indicator of transcoding
	if len(session.TranscodeReasons) > 0 {
		return "Transcode"
	}

	// Check the PlayMethod field from Emby if it exists
	if session.PlayMethod != "" {
		// If it starts with "trans", it's transcoding
		if strings.HasPrefix(strings.ToLower(session.PlayMethod), "trans") {
			return "Transcode"
		}
		// Otherwise treat as direct
		return "DirectPlay"
	}

	// Default to DirectPlay if no indicators of transcoding
	return "DirectPlay"
}

func (w *EmbyWS) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}
