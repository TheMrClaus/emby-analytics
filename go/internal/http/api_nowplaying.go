package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
)

type EmbySession any

// Small struct we send to the UI, keeping your theme logic simple.
type NowPlayingPayload struct {
	HasSession bool            `json:"hasSession"`
	SessionID  string          `json:"sessionId"`
	Raw        json.RawMessage `json:"raw"` // keep the full Emby object available to the UI
}

// Helper to call Emby using env vars EMBY_BASE_URL and EMBY_API_KEY
func embyGET(path string) (*http.Response, error) {
	base := os.Getenv("EMBY_BASE_URL")
	key := os.Getenv("EMBY_API_KEY")
	req, _ := http.NewRequest(http.MethodGet, base+path, nil)
	// Emby accepts either X-Emby-Token or ?api_key=
	req.Header.Set("X-Emby-Token", key)
	c := &http.Client{Timeout: 7 * time.Second}
	return c.Do(req)
}

func RegisterNowPlayingRoutes(app *fiber.App) {
	api := app.Group("/api")

	// GET /api/nowplaying : return the first active session, raw from Emby, plus a couple helper fields
	api.Get("/nowplaying", func(c fiber.Ctx) error {
		// ActiveWithinSeconds keeps the list tight; tweak if you want
		resp, err := embyGET("/Sessions?ActiveWithinSeconds=180")
		if err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
		}
		defer resp.Body.Close()

		var sessions []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "bad Emby JSON: " + err.Error()})
		}

		if len(sessions) == 0 {
			return c.JSON(NowPlayingPayload{HasSession: false})
		}

		// Pick the first session that has NowPlayingItem
		for _, s := range sessions {
			if s["NowPlayingItem"] != nil {
				raw, _ := json.Marshal(s)
				id, _ := s["Id"].(string)
				return c.JSON(NowPlayingPayload{
					HasSession: true,
					SessionID:  id,
					Raw:        raw,
				})
			}
		}
		return c.JSON(NowPlayingPayload{HasSession: false})
	})

	// (Optional) control stubs; keep endpoints stable for your UI.
	api.Post("/sessions/:id/pause", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) })
	api.Post("/sessions/:id/stop", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) })
	api.Post("/sessions/:id/message", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) })
}
