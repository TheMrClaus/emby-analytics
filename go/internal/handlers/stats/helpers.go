package stats

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

// parseQueryInt is now defined only once here.
func parseQueryInt(c fiber.Ctx, key string, def int) int {
	if v := c.Query(key, ""); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// parseTimeframeToDays is also defined only once here.
func parseTimeframeToDays(timeframe string) int {
	switch timeframe {
	case "1d":
		return 1
	case "3d":
		return 3
	case "7d":
		return 7
	case "14d":
		return 14
	case "30d":
		return 30
	case "all-time":
		return 0 // Special case
	default:
		return 14 // Default fallback
	}
}
