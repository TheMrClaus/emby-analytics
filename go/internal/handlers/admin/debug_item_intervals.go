package admin

import (
    "database/sql"
    "strconv"
    "sort"
    "time"

    "github.com/gofiber/fiber/v3"
)

type intervalRow struct {
    SessionFK int64  `json:"session_fk"`
    SessionID string `json:"session_id"`
    Start     int64  `json:"start_ts"`
    End       int64  `json:"end_ts"`
}

type sessionSummary struct {
    SessionFK       int64         `json:"session_fk"`
    SessionID       string        `json:"session_id"`
    Intervals       []intervalRow `json:"intervals"`
    SumSeconds      int64         `json:"sum_seconds"`
    CoalescedSecond int64         `json:"coalesced_seconds"`
}

type itemIntervalsResponse struct {
    ItemID            string           `json:"item_id"`
    WindowStart       int64            `json:"window_start"`
    WindowEnd         int64            `json:"window_end"`
    SessionSummaries  []sessionSummary `json:"sessions"`
    TotalSumSeconds   int64            `json:"total_sum_seconds"`
    TotalCoalescedSec int64            `json:"total_coalesced_seconds"`
}

// GET /admin/debug/item-intervals/:id?days=14
func DebugItemIntervals(db *sql.DB) fiber.Handler {
    return func(c fiber.Ctx) error {
        itemID := c.Params("id", "")
        if itemID == "" {
            return c.Status(400).JSON(fiber.Map{"error": "missing item id"})
        }
        days := 14
        if v := c.Query("days", ""); v != "" {
            if n, err := strconv.Atoi(v); err == nil && n > 0 {
                days = n
            }
        }
        now := time.Now().UTC()
        winEnd := now.Unix()
        winStart := now.AddDate(0, 0, -days).Unix()

        rows, err := db.Query(`
            SELECT pi.session_fk, ps.session_id, pi.start_ts, pi.end_ts
            FROM play_intervals pi
            JOIN play_sessions ps ON ps.id = pi.session_fk
            WHERE pi.item_id = ? AND pi.start_ts <= ? AND pi.end_ts >= ?
            ORDER BY pi.session_fk, pi.start_ts, pi.end_ts
        `, itemID, winEnd, winStart)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        defer rows.Close()

        bySession := make(map[int64]*sessionSummary)
        order := make([]int64, 0)
        var totalSum int64
        for rows.Next() {
            var r intervalRow
            if err := rows.Scan(&r.SessionFK, &r.SessionID, &r.Start, &r.End); err != nil {
                return c.Status(500).JSON(fiber.Map{"error": err.Error()})
            }
            if r.End < r.Start { continue }
            ss, ok := bySession[r.SessionFK]
            if !ok {
                ss = &sessionSummary{SessionFK: r.SessionFK, SessionID: r.SessionID}
                bySession[r.SessionFK] = ss
                order = append(order, r.SessionFK)
            }
            ss.Intervals = append(ss.Intervals, r)
            totalSum += (r.End - r.Start)
        }
        if err := rows.Err(); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }

        // Coalesce per session
        var totalCoalesced int64
        for _, fk := range order {
            ss := bySession[fk]
            sort.Slice(ss.Intervals, func(i, j int) bool {
                if ss.Intervals[i].Start == ss.Intervals[j].Start {
                    return ss.Intervals[i].End < ss.Intervals[j].End
                }
                return ss.Intervals[i].Start < ss.Intervals[j].Start
            })
            // sum of raw seconds across intervals
            ss.SumSeconds = 0
            for _, iv := range ss.Intervals {
                if iv.End > iv.Start {
                    ss.SumSeconds += (iv.End - iv.Start)
                }
            }
            var curS, curE int64
            for i, iv := range ss.Intervals {
                if i == 0 {
                    curS, curE = iv.Start, iv.End
                    continue
                }
                if iv.Start <= curE {
                    if iv.End > curE { curE = iv.End }
                } else {
                    ss.CoalescedSecond += (curE - curS)
                    curS, curE = iv.Start, iv.End
                }
            }
            // close last
            if len(ss.Intervals) > 0 {
                ss.CoalescedSecond += (curE - curS)
            }
            totalCoalesced += ss.CoalescedSecond
        }

        // Build ordered list
        outSessions := make([]sessionSummary, 0, len(order))
        for _, fk := range order {
            outSessions = append(outSessions, *bySession[fk])
        }

        resp := itemIntervalsResponse{
            ItemID:            itemID,
            WindowStart:       winStart,
            WindowEnd:         winEnd,
            SessionSummaries:  outSessions,
            TotalSumSeconds:   totalSum,
            TotalCoalescedSec: totalCoalesced,
        }
        return c.JSON(resp)
    }
}
