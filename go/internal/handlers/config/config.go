package config

import (
	"emby-analytics/internal/logging"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
	plexclient "emby-analytics/internal/plex"

	"github.com/gofiber/fiber/v3"
)

type ConfigResponse struct {
	EmbyExternalURL     string `json:"emby_external_url"`
	EmbyServerID        string `json:"emby_server_id"`
	PlexExternalURL     string `json:"plex_external_url,omitempty"`
	JellyfinExternalURL string `json:"jellyfin_external_url,omitempty"`
	PlexServerID        string `json:"plex_server_id,omitempty"`
}

// GetConfig returns client-safe configuration values including server ID
func GetConfig(cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		response := ConfigResponse{
			EmbyExternalURL:     cfg.EmbyExternalURL,
			EmbyServerID:        "", // default empty
			PlexExternalURL:     "",
			JellyfinExternalURL: "",
			PlexServerID:        "",
		}

		// Try to get server ID from Emby
		em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)
		if systemInfo, err := em.GetSystemInfo(); err != nil {
			logging.Debug("Warning: Failed to fetch Emby server ID: %v", err)
		} else {
			response.EmbyServerID = systemInfo.ID
		}

		// Provide external URLs for Plex/Jellyfin if configured
		for _, sc := range cfg.MediaServers {
			switch sc.Type {
			case "plex":
				response.PlexExternalURL = sc.ExternalURL
				// Try to fetch Plex Machine Identifier
				if sc.Enabled && response.PlexServerID == "" {
					if client := plexclient.New(sc); client != nil {
						if si, err := client.GetSystemInfo(); err == nil && si != nil {
							response.PlexServerID = si.ID
						}
					}
				}
			case "jellyfin":
				response.JellyfinExternalURL = sc.ExternalURL
			}
		}

		return c.JSON(response)
	}
}
