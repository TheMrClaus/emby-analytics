package config

import (
	"emby-analytics/internal/config"

	"github.com/gofiber/fiber/v3"
)

type ConfigResponse struct {
	EmbyExternalURL string `json:"emby_external_url"`
}

// GetConfig returns client-safe configuration values
func GetConfig(cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(ConfigResponse{
			EmbyExternalURL: cfg.EmbyExternalURL,
		})
	}
}
