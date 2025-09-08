package admin

import (
    "emby-analytics/internal/emby"

    "github.com/gofiber/fiber/v3"
)

// DebugFindSeriesID resolves a Series Id by name using Emby search.
// GET /admin/debug/series-id?name=Hostage
func DebugFindSeriesID(em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
        name := c.Query("name", "")
        if name == "" {
            return c.Status(400).JSON(fiber.Map{"error": "missing name"})
        }
        id, err := em.FindSeriesIDByName(name)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(fiber.Map{"name": name, "series_id": id})
    }
}

