package config

import (
	"log"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"

	"github.com/gofiber/fiber/v3"
)

type ConfigResponse struct {
	EmbyExternalURL string `json:"emby_external_url"`
	EmbyServerID    string `json:"emby_server_id"`
}

// GetConfig returns client-safe configuration values including server ID
func GetConfig(cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		response := ConfigResponse{
			EmbyExternalURL: cfg.EmbyExternalURL,
			EmbyServerID:    "", // default empty
		}

		// Try to get server ID from Emby
		em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)
		if systemInfo, err := em.GetSystemInfo(); err != nil {
			log.Printf("[config] Warning: Failed to fetch Emby server ID: %v", err)
		} else {
			response.EmbyServerID = systemInfo.ID
		}

		return c.JSON(response)
	}
}
