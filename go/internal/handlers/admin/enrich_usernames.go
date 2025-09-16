package admin

import (
    "database/sql"
    "strings"

    "github.com/gofiber/fiber/v3"

    "emby-analytics/internal/media"
)

// EnrichUserNames backfills play_sessions.user_name for Plex/Jellyfin using server APIs.
// POST /admin/enrich/user-names
func EnrichUserNames(db *sql.DB, mgr *media.MultiServerManager) fiber.Handler {
    return func(c fiber.Ctx) error {
        if mgr == nil {
            return c.Status(503).JSON(fiber.Map{"error": "multi-server not initialized"})
        }

        // Collect distinct (server_id, user_id) pairs where user_name is missing or equals user_id
        rows, err := db.Query(`
            SELECT DISTINCT server_id, user_id
            FROM play_sessions
            WHERE (user_name IS NULL OR user_name = '' OR user_name = user_id)
              AND COALESCE(server_type,'') IN ('plex','jellyfin')
              AND user_id IS NOT NULL AND user_id <> ''
        `)
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        defer rows.Close()

        type pair struct{ sid, uid string }
        pairs := make([]pair, 0)
        for rows.Next() {
            var sid, uid string
            if err := rows.Scan(&sid, &uid); err == nil {
                if strings.TrimSpace(sid) != "" && strings.TrimSpace(uid) != "" {
                    pairs = append(pairs, pair{sid: sid, uid: uid})
                }
            }
        }

        // Group by server
        byServer := make(map[string][]string)
        for _, p := range pairs { byServer[p.sid] = append(byServer[p.sid], p.uid) }

        updated := 0
        servers := 0
        for sid, uids := range byServer {
            client, ok := mgr.GetClient(sid)
            if !ok || client == nil { continue }
            servers++
            users, err := client.GetUsers()
            if err != nil { continue }
            nameByID := make(map[string]string, len(users))
            for _, u := range users { nameByID[u.ID] = u.Name }
            for _, uid := range uids {
                if name := strings.TrimSpace(nameByID[uid]); name != "" {
                    if _, err := db.Exec(`UPDATE play_sessions SET user_name = ? WHERE server_id = ? AND user_id = ?`, name, sid, uid); err == nil {
                        updated++
                    }
                }
            }
        }

        return c.JSON(fiber.Map{"pairs": len(pairs), "updated": updated, "servers": servers})
    }
}

