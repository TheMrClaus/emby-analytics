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
	Cfg     WSConfig
	conn    *websocket.Conn
	cancel  context.CancelFunc
	Handler func(evt EmbyEvent)
}

type EmbyEvent struct {
	MessageType string          `json:"MessageType"`
	Data        json.RawMessage `json:"Data"`
}

type PlaybackProgressData struct {
	UserID           string   `json:"UserId"`
	SessionID        string   `json:"SessionId"`
	DeviceID         string   `json:"DeviceId"`
	Client           string   `json:"Client"`
	PlayMethod       string   `json:"PlayMethod"`
	RemoteEndPoint   string   `json:"RemoteEndPoint"`
	TranscodeReasons []string `json:"TranscodeReasons"`

	NowPlaying struct {
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
		defer func() {
			if w.conn != nil {
				w.conn.Close()
			}
		}()

		retry := 2 * time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			log.Printf("[emby-ws] Attempting to connect...")
			c, _, err := w.dial()
			if err != nil {
				log.Printf("[emby-ws] dial error: %v", err)
				time.Sleep(retry)
				if retry < 30*time.Second {
					retry *= 2
				}
				continue
			}
			w.conn = c
			retry = 2 * time.Second
			log.Printf("[emby-ws] successfully connected")

			// Identify the client and request updates with debug logging
			log.Printf("[emby-ws] Sending subscription requests...")

			sessionsMsg := `{"MessageType":"SessionsStart", "Data": "0,1000"}`
			playbackMsg := `{"MessageType":"PlaybackStart", "Data": "0,1000"}`

			log.Printf("[emby-ws] Sending: %s", sessionsMsg)
			if err := w.conn.WriteMessage(websocket.TextMessage, []byte(sessionsMsg)); err != nil {
				log.Printf("[emby-ws] Failed to send SessionsStart: %v", err)
			}

			log.Printf("[emby-ws] Sending: %s", playbackMsg)
			if err := w.conn.WriteMessage(websocket.TextMessage, []byte(playbackMsg)); err != nil {
				log.Printf("[emby-ws] Failed to send PlaybackStart: %v", err)
			}

			log.Printf("[emby-ws] Subscriptions sent, starting read loop...")

			// Read loop
			messageCount := 0
			for {
				_, p, err := c.ReadMessage()
				if err != nil {
					log.Printf("[emby-ws] read error: %v", err)
					break
				}

				messageCount++

				// DEBUG: Log all received messages
				msgPreview := string(p)
				if len(msgPreview) > 200 {
					msgPreview = msgPreview[:200] + "..."
				}
				log.Printf("[emby-ws] Message #%d received (%d bytes): %s", messageCount, len(p), msgPreview)

				var evt EmbyEvent
				if err := json.Unmarshal(p, &evt); err != nil {
					log.Printf("[emby-ws] unmarshal error: %v", err)
					continue
				}

				log.Printf("[emby-ws] Parsed event: MessageType='%s'", evt.MessageType)

				// Handle both Playback events AND Sessions events
				if strings.HasPrefix(evt.MessageType, "Playback") {
					log.Printf("[emby-ws] ✅ PLAYBACK EVENT - Calling handler for: %s", evt.MessageType)
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

	for _, session := range sessions {
		if session.NowPlayingItem == nil {
			continue // No active playback
		}

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
				IsPaused      bool    `json:"IsPaused"`
				PositionTicks int64   `json:"PositionTicks"`
				PlaybackRate  float64 `json:"PlaybackRate"`
			}{
				IsPaused:      session.PlayState.IsPaused,
				PositionTicks: session.PlayState.PositionTicks,
				PlaybackRate:  session.PlayState.PlaybackRate,
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

// Update the SessionData struct in ws.go to include stream indices
type SessionData struct {
	UserID           string   `json:"UserId"`
	SessionID        string   `json:"Id"`
	DeviceID         string   `json:"DeviceId"`
	Client           string   `json:"Client"`
	RemoteEndPoint   string   `json:"RemoteEndPoint"`
	TranscodeReasons []string `json:"TranscodeReasons,omitempty"`

	NowPlayingItem *struct {
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

	TranscodingInfo *struct {
		IsVideoDirect bool `json:"IsVideoDirect"`
		IsAudioDirect bool `json:"IsAudioDirect"`
	} `json:"TranscodingInfo"`
}

// Update the handleSessionsEvent function to pass through stream indices
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

	for _, session := range sessions {
		if session.NowPlayingItem == nil {
			continue // No active playback
		}

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
				AudioStreamIndex    *int    `json:"AudioStreamIndex"` // Pass through stream indices
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
		if data, err := json.Marshal(progressData); err == nil {
			syntheticEvent.Data = data
			w.Handler(syntheticEvent)
		}
	}
}

func (w *EmbyWS) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}
