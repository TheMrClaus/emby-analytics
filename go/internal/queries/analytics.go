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
    // Use only the latest interval per session to avoid double-counting
    query := `
        WITH latest AS (
            SELECT pi.*
            FROM play_intervals pi
            JOIN (
                SELECT session_fk, MAX(id) AS latest_id
                FROM play_intervals
                GROUP BY session_fk
            ) m ON m.latest_id = pi.id
        )
        SELECT
            l.user_id,
            u.name,
            SUM(MIN(l.end_ts, ?) - MAX(l.start_ts, ?)) / 3600.0 AS hours
        FROM latest l
        JOIN emby_user u ON u.id = l.user_id
        WHERE
            l.start_ts <= ? AND l.end_ts >= ?
        GROUP BY l.user_id, u.name
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
    // Use only the latest interval per session to avoid double-counting
    query := `
        WITH latest AS (
            SELECT pi.*
            FROM play_intervals pi
            JOIN (
                SELECT session_fk, MAX(id) AS latest_id
                FROM play_intervals
                GROUP BY session_fk
            ) m ON m.latest_id = pi.id
        )
        SELECT
            l.item_id,
            li.name,
            li.media_type,
            SUM(MIN(l.end_ts, ?) - MAX(l.start_ts, ?)) / 3600.0 AS hours
        FROM latest l
        JOIN library_item li ON li.id = l.item_id
        WHERE
            l.start_ts <= ? AND l.end_ts >= ?
        GROUP BY l.item_id, li.name, li.media_type
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
