package admin

import (
	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

// DebugUserHistory shows raw data from Emby for debugging
func DebugUserHistory(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := c.Query("user_id", "")
		days := parseQueryInt(c, "days", 30)

		if userID == "" {
			// If no user specified, get all users first
			users, err := em.GetUsers()
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to get users: " + err.Error()})
			}

			userList := make([]map[string]string, len(users))
			for i, user := range users {
				userList[i] = map[string]string{
					"id":   user.Id,
					"name": user.Name,
				}
			}

			return c.JSON(fiber.Map{
				"message": "Specify user_id parameter. Available users:",
				"users":   userList,
				"example": "/admin/debug/history?user_id=" + users[0].Id + "&days=30",
			})
		}

		// Get user history from Emby
		history, err := em.GetUserPlayHistory(userID, days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to get user history: " + err.Error()})
		}

		return c.JSON(fiber.Map{
			"user_id":     userID,
			"days":        days,
			"total_items": len(history),
			"items":       history,
		})
	}
}

// DebugUserRecentActivity shows ALL recent activity (including partially watched)
func DebugUserRecentActivity(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := c.Query("user_id", "")
		days := parseQueryInt(c, "days", 7)

		if userID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "user_id parameter required"})
		}

		// Make a custom API call without the IsPlayed filter
		items, err := em.GetUserRecentActivity(userID, days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to get recent activity: " + err.Error()})
		}

		return c.JSON(fiber.Map{
			"user_id":     userID,
			"days":        days,
			"total_items": len(items),
			"note":        "This includes ALL recent activity, not just completed items",
			"items":       items,
		})
	}
}
