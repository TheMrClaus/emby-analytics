package now

import (
	"context"
	"log"
	"sync"
	"time"

	"emby-analytics/internal/emby"
	"emby-analytics/internal/logging"

	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

// Broadcaster manages a single Emby API poller and broadcasts to multiple WebSocket clients
type Broadcaster struct {
	mu               sync.RWMutex
	clients          map[*ws.Conn]bool
	embyClient       *emby.Client
	interval         time.Duration
	ctx              context.Context
	cancel           context.CancelFunc
	SessionProcessor func(activeSessions []emby.EmbySession) // NEW: callback for hybrid session processing
}

// NewBroadcaster creates a new broadcaster instance
func NewBroadcaster(embyClient *emby.Client, pollInterval time.Duration) *Broadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	return &Broadcaster{
		clients:    make(map[*ws.Conn]bool),
		embyClient: embyClient,
		interval:   pollInterval,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the polling and broadcasting goroutine
func (b *Broadcaster) Start() {
	go b.broadcastLoop()
}

// Stop shuts down the broadcaster and closes all client connections
func (b *Broadcaster) Stop() {
	b.cancel()

	b.mu.Lock()
	defer b.mu.Unlock()

	for client := range b.clients {
		_ = client.Close()
	}
	b.clients = make(map[*ws.Conn]bool)
}

// AddClient registers a new WebSocket client for broadcasts
func (b *Broadcaster) AddClient(conn *ws.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.clients[conn] = true

	// Send immediate snapshot to new client
	go b.sendToClient(conn)
}

// RemoveClient unregisters a WebSocket client
func (b *Broadcaster) RemoveClient(conn *ws.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.clients, conn)
}

// broadcastLoop is the main polling and broadcasting goroutine
func (b *Broadcaster) broadcastLoop() {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	// Send immediately when started
	b.broadcast()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.broadcast()
		}
	}
}

// broadcast fetches data from Emby and sends to all connected clients
func (b *Broadcaster) broadcast() {
	// MODIFIED: Fetch entries first. If it fails, do not broadcast.
	entries, err := b.fetchNowPlayingEntries()
	if err != nil {
		log.Printf("[broadcaster] failed to fetch now playing data, skipping broadcast: %v", err)
		return // Do nothing on error
	}

	b.mu.RLock()
	clients := make([]*ws.Conn, 0, len(b.clients))
	for client := range b.clients {
		clients = append(clients, client)
	}
	b.mu.RUnlock()

	for _, client := range clients {
		go b.sendToClientWithData(client, entries)
	}
}

// sendToClient sends current data to a specific client
func (b *Broadcaster) sendToClient(conn *ws.Conn) {
	entries, err := b.fetchNowPlayingEntries()
	if err != nil {
		log.Printf("[broadcaster] failed to send initial snapshot to client: %v", err)
		// Don't send anything if the initial fetch fails
		return
	}
	b.sendToClientWithData(conn, entries)
}

func (b *Broadcaster) sendToClientWithData(conn *ws.Conn, entries []NowEntry) {
	if err := conn.WriteJSON(entries); err != nil {
		b.RemoveClient(conn)
		_ = conn.Close()
	}
}

// fetchNowPlayingEntries now returns an error if it fails
func (b *Broadcaster) fetchNowPlayingEntries() ([]NowEntry, error) {
	sessions, err := b.embyClient.GetActiveSessions()
	if err != nil {
		return nil, err // Return the error instead of an empty slice
	}

	// NEW: Process sessions using hybrid state-polling approach (like playback_reporting plugin)
	if b.SessionProcessor != nil {
		logging.Debug("Found active sessions from Emby API", "count", len(sessions))
		b.SessionProcessor(sessions) // Pass the full session list for processing
	}

	nowTime := time.Now().UnixMilli()
	entries := make([]NowEntry, 0, len(sessions))

	for _, s := range sessions {
		var pct float64
		if s.DurationTicks > 0 {
			pct = float64(s.PosTicks) / float64(s.DurationTicks) * 100
		}
		entries = append(entries, NowEntry{
			Timestamp:   nowTime,
			Title:       s.ItemName,
			User:        s.UserName,
			App:         s.App,
			Device:      s.Device,
			PlayMethod:  s.PlayMethod,
			Video:       videoDetailFromSession(s),
			Audio:       audioDetailFromSession(s),
			Subs:        s.SubLang,
			Bitrate:     s.Bitrate,
			ProgressPct: pct,
			PositionSec: func() int64 {
				if s.PosTicks > 0 {
					return s.PosTicks / 10_000_000
				}
				return 0
			}(),
			DurationSec: func() int64 {
				if s.DurationTicks > 0 {
					return s.DurationTicks / 10_000_000
				}
				return 0
			}(),
			Poster:         "/img/primary/" + s.ItemID,
			SessionID:      s.SessionID,
			ItemID:         s.ItemID,
			ItemType:       s.ItemType,
			Container:      s.Container,
			Width:          s.Width,
			Height:         s.Height,
			DolbyVision:    s.DolbyVision,
			HDR10:          s.HDR10,
			AudioLang:      s.AudioLang,
			AudioCh:        s.AudioCh,
			SubLang:        s.SubLang,
			SubCodec:       s.SubCodec,
			TransVideoFrom: s.TransVideoFrom,
			TransVideoTo:   s.TransVideoTo,
			TransAudioFrom: s.TransAudioFrom,
			TransAudioTo:   s.TransAudioTo,
			VideoMethod:    s.VideoMethod,
			AudioMethod:    s.AudioMethod,
			TransReason:    reasonText(s.VideoMethod, s.AudioMethod, s.TransReasons),
			TransPct:       s.TransCompletion,
			IsPaused:       s.IsPaused,
		})
	}

	return entries, nil
}
