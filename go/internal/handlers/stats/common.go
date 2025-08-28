// go/internal/handlers/stats/common.go
package stats

// excludeLiveTvFilter returns the SQL WHERE clause to exclude Live TV content
func excludeLiveTvFilter() string {
	return "li.media_type NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')"
}

// withLiveTvFilter adds Live TV exclusion to an existing WHERE clause
func withLiveTvFilter(existingWhere string) string {
	if existingWhere == "" {
		return "WHERE " + excludeLiveTvFilter()
	}
	return existingWhere + " AND " + excludeLiveTvFilter()
}
