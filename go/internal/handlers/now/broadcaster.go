package now

import (
	"context"
	"log"
	"sync"
	"time"

	"emby-analytics/internal/emby"

	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

// Broadcaster manages a single Emby API poller and broadcasts to multiple WebSocket clients
type Broadcaster struct {
	mu         sync.RWMutex
	clients    map[*ws.Conn]bool
	embyClient *emby.Client
	interval   time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
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

	// Close all client connections
	for client := range b.clients {
		client.Close()
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

// GetClientCount returns the number of active WebSocket clients
func (b *Broadcaster) GetClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.clients)
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
	entries := b.fetchNowPlayingEntries()

	b.mu.RLock()
	clients := make([]*ws.Conn, 0, len(b.clients))
	for client := range b.clients {
		clients = append(clients, client)
	}
	b.mu.RUnlock()

	// Send to all clients (do this outside the lock to avoid blocking)
	for _, client := range clients {
		go b.sendToClientWithData(client, entries)
	}
}

// sendToClient sends current data to a specific client
func (b *Broadcaster) sendToClient(conn *ws.Conn) {
	entries := b.fetchNowPlayingEntries()
	b.sendToClientWithData(conn, entries)
}

// sendToClientWithData sends specific data to a client
func (b *Broadcaster) sendToClientWithData(conn *ws.Conn, entries []NowEntry) {
	if err := conn.WriteJSON(entries); err != nil {
		// Client disconnected, remove it
		b.RemoveClient(conn)
		conn.Close()
	}
}

// fetchNowPlayingEntries fetches and formats current sessions from Emby
func (b *Broadcaster) fetchNowPlayingEntries() []NowEntry {
	sessions, err := b.embyClient.GetActiveSessions()
	if err != nil {
		log.Printf("Error fetching active sessions: %v", err)
		return []NowEntry{} // Return empty list on error
	}

	nowTime := time.Now().UnixMilli()
	entries := make([]NowEntry, 0, len(sessions))

	for _, s := range sessions {
		var pct float64
		if s.DurationTicks > 0 {
			pct = float64(s.PosTicks) / float64(s.DurationTicks) * 100
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
		}

		entries = append(entries, NowEntry{
			Timestamp:         nowTime,
			Title:             s.ItemName,
			User:              s.UserName,
			App:               s.App,
			Device:            s.Device,
			PlayMethod:        s.PlayMethod,
			Video:             videoDetailFromSession(s),
			Audio:             audioDetailFromSession(s),
			Subs:              s.SubLang,
			Bitrate:           s.Bitrate,
			ProgressPct:       pct,
			Poster:            "/img/primary/" + s.ItemID,
			SessionID:         s.SessionID,
			ItemID:            s.ItemID,
			ItemType:          s.ItemType,
			Container:         s.Container,
			Width:             s.Width,
			Height:            s.Height,
			DolbyVision:       s.DolbyVision,
			HDR10:             s.HDR10,
			AudioLang:         s.AudioLang,
			AudioCh:           s.AudioCh,
			SubLang:           s.SubLang,
			SubCodec:          s.SubCodec,
			StreamPath:        streamPathLabel(s.Container),
			StreamDetail:      mbps(s.Bitrate),
			TransReason:       reasonText(s.VideoMethod, s.AudioMethod, s.TransReasons),
			TransPct:          s.TransCompletion,
			TransAudioBitrate: s.TransAudioBitrate,
			TransVideoBitrate: s.TransVideoBitrate,
			TransVideoFrom:    s.TransVideoFrom,
			TransVideoTo:      s.TransVideoTo,
			TransAudioFrom:    s.TransAudioFrom,
			TransAudioTo:      s.TransAudioTo,
			VideoMethod:       s.VideoMethod,
			AudioMethod:       s.AudioMethod,
		})
	}

	return entries
}
