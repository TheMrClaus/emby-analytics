package admin

import (
	"emby-analytics/internal/emby"
	"github.com/gofiber/fiber/v3"
)

// DebugUsers tests the GetUsers API call directly
func DebugUsers(em *emby.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		users, err := em.GetUsers()
		if err != nil {
			return c.JSON(fiber.Map{
				"success": false,
				"error": err.Error(),
				"api_url": em.BaseURL + "/emby/Users",
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"user_count": len(users),
			"users": users,
			"api_url": em.BaseURL + "/emby/Users",
		})
	}
}
