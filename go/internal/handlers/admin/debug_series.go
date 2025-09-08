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

// DebugSeriesFromEpisode returns the SeriesId for a given episode id via Emby lookup.
// GET /admin/debug/series-from-episode?id=EPISODE_ID
func DebugSeriesFromEpisode(em *emby.Client) fiber.Handler {
    return func(c fiber.Ctx) error {
        id := c.Query("id", "")
        if id == "" {
            return c.Status(400).JSON(fiber.Map{"error": "missing id"})
        }
        if em == nil {
            return c.Status(500).JSON(fiber.Map{"error": "Emby client not configured"})
        }
        items, err := em.ItemsByIDs([]string{id})
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        if len(items) == 0 {
            return c.JSON(fiber.Map{"episode_id": id, "series_id": "", "series_name": ""})
        }
        it := items[0]
        return c.JSON(fiber.Map{"episode_id": id, "series_id": it.SeriesId, "series_name": it.SeriesName, "type": it.Type, "name": it.Name})
    }
}
