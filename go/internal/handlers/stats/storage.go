package stats

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

// StaleContentItem represents an item with zero plays
type StaleContentItem struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	ItemType string  `json:"item_type"`
	ServerID string  `json:"server_id"`
	SizeGB   float64 `json:"size_gb"`
	AddedAt  string  `json:"added_at,omitempty"`
}

// ROIItem represents watch hours per GB analysis
type ROIItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ItemType   string  `json:"item_type"`
	ServerID   string  `json:"server_id"`
	PlayCount  int     `json:"play_count"`
	WatchHours float64 `json:"watch_hours"`
	SizeGB     float64 `json:"size_gb"`
	HoursPerGB float64 `json:"hours_per_gb"`
}

// DuplicateGroup represents files with the same normalized path
type DuplicateGroup struct {
	NormalizedPath string   `json:"normalized_path"`
	DuplicateCount int      `json:"duplicate_count"`
	TotalSizeGB    float64  `json:"total_size_gb"`
	ItemIDs        []string `json:"item_ids"`
	Titles         []string `json:"titles"`
}

// StoragePrediction represents storage growth forecast
type StoragePrediction struct {
	HistoricalData   []SnapshotData  `json:"historical_data"`
	Predictions      []PredictedData `json:"predictions"`
	CurrentSizeGB    float64         `json:"current_size_gb"`
	ProjectedSizeGB  float64         `json:"projected_size_gb_6mo"`
	GrowthRatePerDay float64         `json:"growth_rate_gb_per_day"`
}

type SnapshotData struct {
	Date   string  `json:"date"`
	SizeGB float64 `json:"size_gb"`
}

type PredictedData struct {
	Date   string  `json:"date"`
	SizeGB float64 `json:"size_gb"`
}

// StaleContent returns items with zero plays, sorted by size (largest first)
func StaleContent(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 100)
		if limit <= 0 || limit > 1000 {
			limit = 100
		}

		rows, err := db.Query(`
			SELECT 
				li.id,
				li.name,
				li.media_type,
				li.server_id,
				li.file_size_bytes / 1073741824.0 as size_gb,
				li.created_at
			FROM library_item li
			LEFT JOIN play_sessions ps ON li.id = ps.item_id
			WHERE ps.id IS NULL 
				AND li.file_size_bytes > 0
			GROUP BY li.id
			ORDER BY li.file_size_bytes DESC
			LIMIT ?
		`, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		items := []StaleContentItem{}
		for rows.Next() {
			var item StaleContentItem
			if err := rows.Scan(&item.ID, &item.Title, &item.ItemType, &item.ServerID, &item.SizeGB, &item.AddedAt); err != nil {
				continue
			}
			items = append(items, item)
		}

		return c.JSON(fiber.Map{
			"stale_items": items,
			"total_count": len(items),
		})
	}
}

// ROIAnalysis returns watch hours per GB analysis
func ROIAnalysis(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 100)
		if limit <= 0 || limit > 1000 {
			limit = 100
		}

		// Get sort order: 'best' (highest hours/GB) or 'worst' (lowest hours/GB)
		sortOrder := c.Query("sort", "worst")
		orderBy := "hours_per_gb ASC" // worst first (candidates for deletion)
		if sortOrder == "best" {
			orderBy = "hours_per_gb DESC"
		}

		query := fmt.Sprintf(`
			SELECT 
				li.id,
				li.name,
				li.media_type,
				li.server_id,
				COUNT(DISTINCT pi.id) as play_count,
				COALESCE(SUM(pi.duration_seconds), 0) / 3600.0 as total_watch_hours,
				li.file_size_bytes / 1073741824.0 as size_gb,
				(COALESCE(SUM(pi.duration_seconds), 0) / 3600.0) / (li.file_size_bytes / 1073741824.0) as hours_per_gb
			FROM library_item li
			JOIN play_intervals pi ON li.id = pi.item_id
			WHERE li.file_size_bytes > 0
			GROUP BY li.id
			HAVING play_count > 0
			ORDER BY %s
			LIMIT ?
		`, orderBy)

		rows, err := db.Query(query, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		items := []ROIItem{}
		for rows.Next() {
			var item ROIItem
			if err := rows.Scan(&item.ID, &item.Title, &item.ItemType, &item.ServerID, &item.PlayCount, &item.WatchHours, &item.SizeGB, &item.HoursPerGB); err != nil {
				continue
			}
			items = append(items, item)
		}

		return c.JSON(fiber.Map{
			"roi_items":   items,
			"total_count": len(items),
			"sort":        sortOrder,
		})
	}
}

// Duplicates returns items with duplicate normalized file paths
func Duplicates(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		limit := parseQueryInt(c, "limit", 50)
		if limit <= 0 || limit > 500 {
			limit = 50
		}

		rows, err := db.Query(`
			SELECT 
				LOWER(REPLACE(REPLACE(file_path, '\', '/'), '//', '/')) as normalized_path,
				COUNT(*) as duplicate_count,
				SUM(file_size_bytes) / 1073741824.0 as total_size_gb,
				GROUP_CONCAT(id) as item_ids,
				GROUP_CONCAT(name) as titles
			FROM library_item
			WHERE file_path IS NOT NULL
			GROUP BY normalized_path
			HAVING duplicate_count > 1
			ORDER BY total_size_gb DESC
			LIMIT ?
		`, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		groups := []DuplicateGroup{}
		for rows.Next() {
			var group DuplicateGroup
			var itemIDsStr, titlesStr string
			if err := rows.Scan(&group.NormalizedPath, &group.DuplicateCount, &group.TotalSizeGB, &itemIDsStr, &titlesStr); err != nil {
				continue
			}
			// Parse comma-separated IDs and titles (SQLite GROUP_CONCAT returns comma-separated string)
			group.ItemIDs = parseCommaSeparated(itemIDsStr)
			group.Titles = parseCommaSeparated(titlesStr)
			groups = append(groups, group)
		}

		return c.JSON(fiber.Map{
			"duplicate_groups": groups,
			"total_groups":     len(groups),
		})
	}
}

// StoragePredictions returns historical data and 6-month forecast
func StoragePredictions(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get historical snapshots (last 90 days or all available)
		rows, err := db.Query(`
			SELECT 
				snapshot_date,
				total_size_bytes / 1073741824.0 as size_gb
			FROM library_snapshots
			ORDER BY snapshot_date ASC
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		historical := []SnapshotData{}
		for rows.Next() {
			var snapshot SnapshotData
			if err := rows.Scan(&snapshot.Date, &snapshot.SizeGB); err != nil {
				continue
			}
			historical = append(historical, snapshot)
		}

		// No snapshots at all — nothing to show
		if len(historical) == 0 {
			return c.JSON(fiber.Map{
				"historical_data":        historical,
				"predictions":            []PredictedData{},
				"current_size_gb":        0.0,
				"projected_size_gb_6mo":  0.0,
				"growth_rate_gb_per_day": 0.0,
				"message":               "No snapshots recorded yet. Storage data will appear after the first snapshot is captured.",
			})
		}

		currentSize := historical[len(historical)-1].SizeGB

		// Only 1 snapshot — show current size but can't project growth
		if len(historical) < 2 {
			return c.JSON(fiber.Map{
				"historical_data":        historical,
				"predictions":            []PredictedData{},
				"current_size_gb":        currentSize,
				"projected_size_gb_6mo":  0.0,
				"growth_rate_gb_per_day": 0.0,
				"message":               "Only 1 snapshot available. Growth predictions require at least 2 daily snapshots.",
			})
		}

		// Build accuracy warning for small sample sizes
		var warningMessage string
		if len(historical) < 7 {
			warningMessage = fmt.Sprintf("Based on %d snapshots. Predictions will become more accurate as more daily snapshots are collected.", len(historical))
		}

		// Calculate linear regression for prediction
		// Convert dates to days since first snapshot
		firstDate, _ := time.Parse("2006-01-02", historical[0].Date)
		dataPoints := make([][2]float64, len(historical))
		for i, snapshot := range historical {
			date, _ := time.Parse("2006-01-02", snapshot.Date)
			daysSinceFirst := date.Sub(firstDate).Hours() / 24.0
			dataPoints[i] = [2]float64{daysSinceFirst, snapshot.SizeGB}
		}

		// Linear regression: y = mx + b
		slope, intercept := linearRegression(dataPoints)

		// Generate predictions for next 6 months (180 days)
		lastDate, _ := time.Parse("2006-01-02", historical[len(historical)-1].Date)
		predictions := []PredictedData{}
		for i := 1; i <= 180; i += 7 { // Weekly predictions
			futureDate := lastDate.AddDate(0, 0, i)
			daysSinceFirst := futureDate.Sub(firstDate).Hours() / 24.0
			predictedSize := slope*daysSinceFirst + intercept
			predictions = append(predictions, PredictedData{
				Date:   futureDate.Format("2006-01-02"),
				SizeGB: predictedSize,
			})
		}

		// Calculate 6-month (180 days) projection
		futureDate180 := lastDate.AddDate(0, 6, 0)
		daysSinceFirst180 := futureDate180.Sub(firstDate).Hours() / 24.0
		projected6mo := slope*daysSinceFirst180 + intercept

		resp := fiber.Map{
			"historical_data":        historical,
			"predictions":            predictions,
			"current_size_gb":        currentSize,
			"projected_size_gb_6mo":  projected6mo,
			"growth_rate_gb_per_day": slope,
		}
		if warningMessage != "" {
			resp["message"] = warningMessage
		}
		return c.JSON(resp)
	}
}

// Helper: linear regression for 2D points
func linearRegression(points [][2]float64) (slope, intercept float64) {
	n := float64(len(points))
	if n == 0 {
		return 0, 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range points {
		x, y := p[0], p[1]
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept = (sumY - slope*sumX) / n
	return
}

// Helper: parse comma-separated string into slice
func parseCommaSeparated(s string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
