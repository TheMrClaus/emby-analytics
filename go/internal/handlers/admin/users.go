package admin

import (
	"database/sql"

	"github.com/gofiber/fiber/v3"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
)

// POST /admin/users/sync -> { started: true }
func UsersSyncHandler(db *sql.DB, em *emby.Client, cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Run dedicated user sync, not general sync
		go runUserSyncOnce(db, em)
		return c.JSON(fiber.Map{"started": true})
	}
}

func runUserSyncOnce(db *sql.DB, em *emby.Client) {
	users, err := em.GetUsers()
	if err != nil {
		return
	}

	for _, user := range users {
		_, _ = db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
	}
}
