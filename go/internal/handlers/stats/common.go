// go/internal/handlers/stats/common.go
package stats

import (
	"fmt"
	"strings"
)

// columnWithAlias helps qualify a column with a table alias when needed.
func columnWithAlias(alias, column string) string {
	if strings.TrimSpace(alias) == "" {
		return column
	}
	return fmt.Sprintf("%s.%s", alias, column)
}

// excludeLiveTvFilter returns a SQL fragment to exclude Live TV content.
// It deliberately avoids a table alias so it can be used in queries
// with or without an alias for library_item.
func excludeLiveTvFilter() string {
	return excludeLiveTvFilterAlias("")
}

// excludeLiveTvFilterAlias returns the live TV exclusion predicate, qualifying the column with the provided alias.
func excludeLiveTvFilterAlias(alias string) string {
	col := columnWithAlias(alias, "media_type")
	return fmt.Sprintf("%s NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')", col)
}

// normalizedMediaTypeExpr returns a CASE expression that collapses assorted media_type values
// (or missing metadata) into the buckets used by the UI.
func normalizedMediaTypeExpr(alias string) string {
	mediaTypeCol := columnWithAlias(alias, "media_type")
	seriesIDCol := columnWithAlias(alias, "series_id")
	return fmt.Sprintf(`CASE
        WHEN TRIM(COALESCE(%s, '')) = '' AND TRIM(COALESCE(%s, '')) <> '' THEN 'Episode'
        WHEN LOWER(TRIM(%s)) IN ('episode','season','series') THEN 'Episode'
        WHEN LOWER(TRIM(%s)) = 'movie' THEN 'Movie'
        ELSE 'Unknown'
    END`, mediaTypeCol, seriesIDCol, mediaTypeCol, mediaTypeCol)
}

// withLiveTvFilter adds Live TV exclusion to an existing WHERE clause
func withLiveTvFilter(existingWhere string) string {
	if existingWhere == "" {
		return "WHERE " + excludeLiveTvFilter()
	}
	return existingWhere + " AND " + excludeLiveTvFilter()
}
