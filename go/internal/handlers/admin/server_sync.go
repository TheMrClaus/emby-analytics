package admin

import (
	"database/sql"

	"emby-analytics/internal/config"
	"emby-analytics/internal/logging"
	"emby-analytics/internal/media"
	"emby-analytics/internal/tasks"

	"github.com/gofiber/fiber/v3"
)

// SyncAllServers triggers an immediate background sync for all enabled media servers.
func SyncAllServers(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		go func() {
			tasks.RunOnce(db, mgr, cfg)
		}()
		return c.JSON(fiber.Map{"started": true})
	}
}

// SyncServer triggers a background sync for a single server.
func SyncServer(db *sql.DB, mgr *media.MultiServerManager, cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		serverID := c.Params("id")
		if serverID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "server id is required"})
		}
		configs := mgr.GetServerConfigs()
		if _, ok := configs[serverID]; !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "server not found"})
		}
		go func() {
			if err := tasks.RunServerOnce(db, mgr, cfg, serverID); err != nil {
				logging.Debug("manual server sync failed", "server_id", serverID, "error", err)
			}
		}()
		return c.JSON(fiber.Map{"started": true})
	}
}
