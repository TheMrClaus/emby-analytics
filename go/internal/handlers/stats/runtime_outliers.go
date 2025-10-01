package stats

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type RuntimeOutlier struct {
	LibraryID      string `json:"library_id"`
	ServerID       string `json:"server_id,omitempty"`
	ServerType     string `json:"server_type,omitempty"`
	ItemID         string `json:"item_id"`
	Name           string `json:"name"`
	RuntimeMinutes int    `json:"runtime_minutes"`
	RuntimeHours   string `json:"runtime_hours"`
	RuntimeTicks   int64  `json:"runtime_ticks"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type RuntimeOutliersResponse struct {
	ThresholdMinutes int              `json:"threshold_minutes"`
	HasMore          bool             `json:"has_more"`
	Items            []RuntimeOutlier `json:"items"`
}

func RuntimeOutliers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 50)
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}

		query := `
        SELECT
            id,
            COALESCE(server_id, ''),
            COALESCE(server_type, ''),
            COALESCE(item_id, ''),
            COALESCE(name, ''),
            CAST(run_time_ticks / 600000000 AS INTEGER) AS runtime_minutes,
            run_time_ticks,
            COALESCE(created_at, ''),
            COALESCE(updated_at, '')
        FROM library_item
        WHERE run_time_ticks IS NOT NULL
          AND run_time_ticks / 600000000 > ?
        ORDER BY run_time_ticks DESC
        LIMIT ?
    `

		rows, err := db.Query(query, runtimeOutlierThresholdMinutes, limit+1)
		if err != nil {
			log.Printf("[stats] runtime outlier query failed: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load runtime outliers"})
		}
		defer rows.Close()

		items := make([]RuntimeOutlier, 0)
		for rows.Next() {
			var item RuntimeOutlier
			if err := rows.Scan(&item.LibraryID, &item.ServerID, &item.ServerType, &item.ItemID, &item.Name, &item.RuntimeMinutes, &item.RuntimeTicks, &item.CreatedAt, &item.UpdatedAt); err != nil {
				log.Printf("[stats] failed to scan runtime outlier: %v", err)
				continue
			}
			item.ServerID = strings.TrimSpace(item.ServerID)
			item.ServerType = strings.TrimSpace(item.ServerType)
			item.RuntimeHours = formatRuntimeHours(item.RuntimeMinutes)
			items = append(items, item)
		}

		hasMore := false
		if len(items) > limit {
			hasMore = true
			items = items[:limit]
		}

		return c.JSON(RuntimeOutliersResponse{
			ThresholdMinutes: runtimeOutlierThresholdMinutes,
			HasMore:          hasMore,
			Items:            items,
		})
	}
}

func formatRuntimeHours(minutes int) string {
	if minutes <= 0 {
		return "0m"
	}
	h := minutes / 60
	m := minutes % 60
	if h <= 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
