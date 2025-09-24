package queries

import (
	"context"
	"database/sql"
)

type TopUserRow struct {
	UserID string  `json:"user_id"`
	Name   string  `json:"name"`
	Hours  float64 `json:"hours"`
}

type TopItemRow struct {
	ItemID  string  `json:"item_id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Hours   float64 `json:"hours"`
	Display string  `json:"display"`
}

// TopUsersByWatchSeconds calculates top users based on interval overlap in a time window.
func TopUsersByWatchSeconds(ctx context.Context, db *sql.DB, winStart, winEnd int64, limit int) ([]TopUserRow, error) {
	// Sum overlapped duration across all intervals in the window
	query := `
        SELECT
            l.user_id,
            u.name,
            SUM(
                MAX(
                    0,
                    MIN(
                        MIN(l.end_ts, ?) - MAX(l.start_ts, ?),
                        CASE WHEN l.duration_seconds IS NULL OR l.duration_seconds <= 0
                             THEN (l.end_ts - l.start_ts)
                             ELSE l.duration_seconds
                        END
                    )
                )
            ) / 3600.0 AS hours
        FROM play_intervals l
        JOIN emby_user u ON u.id = l.user_id
        JOIN library_item li ON li.id = l.item_id
        WHERE
            l.start_ts <= ? AND l.end_ts >= ?
            AND COALESCE(li.media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')
        GROUP BY l.user_id, u.name
        HAVING hours > 0
        ORDER BY hours DESC
        LIMIT ?;
    `
	rows, err := db.QueryContext(ctx, query, winEnd, winStart, winEnd, winStart, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TopUserRow
	for rows.Next() {
		var r TopUserRow
		if err := rows.Scan(&r.UserID, &r.Name, &r.Hours); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TopItemsByWatchSeconds calculates top items based on interval overlap.
func TopItemsByWatchSeconds(ctx context.Context, db *sql.DB, winStart, winEnd int64, limit int) ([]TopItemRow, error) {
	// Sum overlapped duration across all intervals in the window
	query := `
        SELECT
            l.item_id,
            li.name,
            li.media_type,
            SUM(
                MAX(
                    0,
                    MIN(
                        MIN(l.end_ts, ?) - MAX(l.start_ts, ?),
                        CASE WHEN l.duration_seconds IS NULL OR l.duration_seconds <= 0
                             THEN (l.end_ts - l.start_ts)
                             ELSE l.duration_seconds
                        END
                    )
                )
            ) / 3600.0 AS hours
        FROM play_intervals l
        JOIN library_item li ON li.id = l.item_id
        WHERE
            l.start_ts <= ? AND l.end_ts >= ?
            AND COALESCE(li.media_type, 'Unknown') NOT IN ('TvChannel', 'LiveTv', 'Channel', 'TvProgram')
        GROUP BY l.item_id, li.name, li.media_type
        HAVING hours > 0
        ORDER BY hours DESC
        LIMIT ?;
    `
	rows, err := db.QueryContext(ctx, query, winEnd, winStart, winEnd, winStart, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TopItemRow
	for rows.Next() {
		var r TopItemRow
		if err := rows.Scan(&r.ItemID, &r.Name, &r.Type, &r.Hours); err != nil {
			return nil, err
		}
		r.Display = r.Name // You can add your episode enrichment logic here later
		out = append(out, r)
	}
	return out, rows.Err()
}
