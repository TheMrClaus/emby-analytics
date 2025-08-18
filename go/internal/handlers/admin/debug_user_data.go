package admin

import (
	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

// DebugUserData shows what Emby returns for user data
func DebugUserData(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := c.Query("user_id")
		if userID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "user_id parameter required"})
		}

		userDataItems, err := em.GetUserData(userID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Calculate totals for debugging
		var totalWatchMs int64
		playedCount := 0
		for _, item := range userDataItems {
			if item.UserData.Played && item.RunTimeTicks > 0 {
				itemRuntimeMs := item.RunTimeTicks / 10000
				totalWatchMs += itemRuntimeMs
				playedCount++
			}
		}

		return c.JSON(fiber.Map{
			"user_id":           userID,
			"total_items":       len(userDataItems),
			"played_items":      playedCount,
			"total_watch_hours": totalWatchMs / 3600000,
			"total_watch_days":  totalWatchMs / (24 * 3600000),
			"sample_items":      userDataItems[:min(5, len(userDataItems))],
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
