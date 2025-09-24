package admin

import (
	emby "emby-analytics/internal/emby"
	"github.com/gofiber/fiber/v3"
)

type EmbySessionDebug struct {
	SessionID   string   `json:"session_id"`
	UserID      string   `json:"user_id"`
	UserName    string   `json:"user_name"`
	ItemID      string   `json:"item_id"`
	Title       string   `json:"title"`
	ItemType    string   `json:"item_type"`
	Client      string   `json:"client"`
	Device      string   `json:"device"`
	PlayMethod  string   `json:"play_method"`
	VideoMethod string   `json:"video_method"`
	AudioMethod string   `json:"audio_method"`
	Reasons     []string `json:"reasons"`
}

// DebugEmbySessions returns the current active sessions as seen from Emby.
func DebugEmbySessions(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		sessions, err := em.GetActiveSessions()
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": err.Error()})
		}
		out := make([]EmbySessionDebug, 0, len(sessions))
		for _, s := range sessions {
			out = append(out, EmbySessionDebug{
				SessionID:   s.SessionID,
				UserID:      s.UserID,
				UserName:    s.UserName,
				ItemID:      s.ItemID,
				Title:       s.ItemName,
				ItemType:    s.ItemType,
				Client:      s.App,
				Device:      s.Device,
				PlayMethod:  s.PlayMethod,
				VideoMethod: s.VideoMethod,
				AudioMethod: s.AudioMethod,
				Reasons:     s.TransReasons,
			})
		}
		return c.JSON(out)
	}
}
