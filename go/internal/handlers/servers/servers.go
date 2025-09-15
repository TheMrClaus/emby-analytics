package servers

import (
    "github.com/gofiber/fiber/v3"
    "emby-analytics/internal/media"
)

var mgr *media.MultiServerManager

// SetManager sets the multi-server manager
func SetManager(m *media.MultiServerManager) { mgr = m }

// List returns configured servers with health status
func List() fiber.Handler {
    return func(c fiber.Ctx) error {
        if mgr == nil {
            return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "multi-server not initialized"})
        }
        cfgs := mgr.GetServerConfigs()
        health := mgr.GetServerHealth()
        type serverOut struct {
            ID        string                 `json:"id"`
            Type      media.ServerType       `json:"type"`
            Name      string                 `json:"name"`
            Enabled   bool                   `json:"enabled"`
            Health    *media.ServerHealth    `json:"health"`
        }
        out := make([]serverOut, 0, len(cfgs))
        for id, cfg := range cfgs {
            out = append(out, serverOut{
                ID:      id,
                Type:    cfg.Type,
                Name:    cfg.Name,
                Enabled: cfg.Enabled,
                Health:  health[id],
            })
        }
        return c.JSON(out)
    }
}

