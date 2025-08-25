package now

import (
	"emby-analytics/internal/broadcaster"

	"github.com/gofiber/fiber/v3"
	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

// Global broadcaster instance (will be set by main.go)
var nowBroadcaster *broadcaster.NowBroadcaster

// SetBroadcaster sets the global broadcaster instance
func SetBroadcaster(b *broadcaster.NowBroadcaster) {
	nowBroadcaster = b
}

// WS returns a Fiber v3 handler that upgrades to WebSocket and connects to the broadcaster
func WS() fiber.Handler {
	return ws.New(func(conn *ws.Conn) {
		defer func() {
			// Remove client from broadcaster when connection closes
			if nowBroadcaster != nil {
				nowBroadcaster.RemoveClient(conn)
			}
			conn.Close()
		}()

		// Add client to broadcaster
		if nowBroadcaster != nil {
			nowBroadcaster.AddClient(conn)
		}

		// Keep connection alive by listening for close
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break // Connection closed
			}
		}
	})
}
