package stats

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func parseQueryInt(c fiber.Ctx, key string, def int) int {
	if v := c.Query(key, ""); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
