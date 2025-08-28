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
		IsPaused      bool    `json:"IsPaused"`
		PositionTicks int64   `json:"PositionTicks"`
		PlaybackRate  float64 `json:"PlaybackRate"`
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

			// Identify the client and request updates
			_ = w.conn.WriteMessage(websocket.TextMessage, []byte(`{"MessageType":"SessionsStart", "Data": "0,1000"}`))
			_ = w.conn.WriteMessage(websocket.TextMessage, []byte(`{"MessageType":"PlaybackStart", "Data": "0,1000"}`))

			// Read loop
			for {
				_, p, err := c.ReadMessage()
				if err != nil {
					log.Printf("[emby-ws] read error: %v", err)
					break
				}
				var evt EmbyEvent
				if err := json.Unmarshal(p, &evt); err != nil {
					log.Printf("[emby-ws] unmarshal error: %v", err)
					continue
				}

				// We only care about playback events
				if strings.HasPrefix(evt.MessageType, "Playback") {
					if w.Handler != nil {
						w.Handler(evt)
					}
				}
			}
			// Reconnect on break
			time.Sleep(retry)
		}
	}()
}

func (w *EmbyWS) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}
