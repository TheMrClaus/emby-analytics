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
	query := `
		SELECT
			pi.user_id,
			u.name,
			SUM(MIN(pi.end_ts, ?) - MAX(pi.start_ts, ?)) / 3600.0 AS hours
		FROM play_intervals pi
		JOIN emby_user u ON u.id = pi.user_id
		WHERE
			pi.start_ts <= ? AND pi.end_ts >= ? -- Filter for intervals that overlap the window
		GROUP BY pi.user_id, u.name
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
	query := `
		SELECT
			pi.item_id,
			li.name,
			li.media_type,
			SUM(MIN(pi.end_ts, ?) - MAX(pi.start_ts, ?)) / 3600.0 AS hours
		FROM play_intervals pi
		JOIN library_item li ON li.id = pi.item_id
		WHERE
			pi.start_ts <= ? AND pi.end_ts >= ? -- Filter for intervals that overlap the window
		GROUP BY pi.item_id, li.name, li.type
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
