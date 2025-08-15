package stats

import (
	"strconv"
	"strings"

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

func parseWindowDays(raw string, defDays int) int {
	if strings.TrimSpace(raw) == "" {
		return defDays
	}
	// supports "14d" or "4w"
	s := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasSuffix(s, "d") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && n > 0 {
			return n
		}
	}
	if strings.HasSuffix(s, "w") {
		if n, err := strconv.Atoi(strings.TrimSuffix(s, "w")); err == nil && n > 0 {
			return n * 7
		}
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return defDays
}
