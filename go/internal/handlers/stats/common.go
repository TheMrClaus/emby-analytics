// go/internal/handlers/stats/common.go
package stats

// excludeLiveTvFilter returns a SQL fragment to exclude Live TV content.
// It deliberately avoids a table alias so it can be used in queries
// with or without an alias for library_item.
func excludeLiveTvFilter() string {
	return "media_type NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')"
}

// withLiveTvFilter adds Live TV exclusion to an existing WHERE clause
func withLiveTvFilter(existingWhere string) string {
	if existingWhere == "" {
		return "WHERE " + excludeLiveTvFilter()
	}
	return existingWhere + " AND " + excludeLiveTvFilter()
}
