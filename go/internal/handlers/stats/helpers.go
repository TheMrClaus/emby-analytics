package stats

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// parseQueryInt is now defined only once here.
func parseQueryInt(c fiber.Ctx, key string, def int) int {
	if v := c.Query(key, ""); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// parseTimeframeToDays is also defined only once here.
func parseTimeframeToDays(timeframe string) int {
	switch timeframe {
	case "1d":
		return 1
	case "3d":
		return 3
	case "7d":
		return 7
	case "14d":
		return 14
	case "30d":
		return 30
	case "all-time":
		return 0 // Special case
	default:
		return 14 // Default fallback
	}
}

func normalizeServerParam(raw string) (serverType string, serverID string) {
	v := strings.TrimSpace(raw)
	if v == "" || strings.EqualFold(v, "all") {
		return "", ""
	}
	lower := strings.ToLower(v)
	switch lower {
	case "emby", "plex", "jellyfin":
		return lower, ""
	default:
		return "", v
	}
}

func serverPredicate(alias string, serverType, serverID string) (string, []interface{}) {
	if serverType == "" && serverID == "" {
		return "", nil
	}
	column := func(col string) string {
		if strings.TrimSpace(alias) == "" {
			return col
		}
		return fmt.Sprintf("%s.%s", alias, col)
	}
	if serverType != "" {
		return fmt.Sprintf("LOWER(COALESCE(%s, '')) = ?", column("server_type")), []interface{}{serverType}
	}
	return fmt.Sprintf("%s = ?", column("server_id")), []interface{}{serverID}
}

func appendServerFilter(baseCondition, alias, serverType, serverID string) (string, []interface{}) {
	predicate, args := serverPredicate(alias, serverType, serverID)
	if predicate == "" {
		return baseCondition, nil
	}
	if strings.TrimSpace(baseCondition) == "" {
		return predicate, args
	}
	return baseCondition + " AND " + predicate, args
}

// normalizedFilePathExpr returns SQL expression for normalizing file paths for deduplication
// Strips common library folder prefixes (/movies/, /tv/, /shows/) to deduplicate across servers
func normalizedFilePathExpr(alias string) string {
	col := "file_path"
	if alias != "" {
		col = alias + ".file_path"
	}
	normalizedCol := fmt.Sprintf("LOWER(REPLACE(%s, '\\', '/'))", col)
	return fmt.Sprintf(`COALESCE(
		NULLIF(
			CASE WHEN INSTR(%s, '/movies/') > 0
				THEN SUBSTR(%s, INSTR(%s, '/movies/') + LENGTH('/movies/'))
				ELSE NULL END,
			''
		),
		NULLIF(
			CASE WHEN INSTR(%s, '/tv/') > 0
				THEN SUBSTR(%s, INSTR(%s, '/tv/') + LENGTH('/tv/'))
				ELSE NULL END,
			''
		),
		NULLIF(
			CASE WHEN INSTR(%s, '/shows/') > 0
				THEN SUBSTR(%s, INSTR(%s, '/shows/') + LENGTH('/shows/'))
				ELSE NULL END,
			''
		),
		%s
	)`, 
		normalizedCol, normalizedCol, normalizedCol,
		normalizedCol, normalizedCol, normalizedCol,
		normalizedCol, normalizedCol, normalizedCol,
		col)
}
