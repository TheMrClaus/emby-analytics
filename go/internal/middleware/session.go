package middleware

import (
	"database/sql"
	"strings"

	"emby-analytics/internal/config"

	"github.com/gofiber/fiber/v3"
)

type userCtx struct {
	ID       int64
	Username string
	Role     string
}

const userLocalsKey = "app_user"

// AttachUser parses the auth cookie and attaches the user (if valid) to context locals.
func AttachUser(db *sql.DB, cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Cookies(cfg.AuthCookieName)
		if token != "" {
			var id int64
			var username, role string
			var count int
			err := db.QueryRow(`
                SELECT u.id, u.username, u.role, COUNT(*)
                FROM app_session s JOIN app_user u ON u.id = s.user_id
                WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
            `, token).Scan(&id, &username, &role, &count)
			if err == nil && count > 0 {
				c.Locals(userLocalsKey, &userCtx{ID: id, Username: username, Role: role})
			}
		}
		return c.Next()
	}
}

// RequireUserForUI ensures UI pages are accessed by authenticated users. It should be applied
// to non-API GET routes before static file serving. Excludes /login and /auth/*.
func RequireUserForUI(cfg config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}
		path := c.Path()
		if strings.HasPrefix(path, "/auth") || path == "/login" || strings.HasPrefix(path, "/health") {
			return c.Next()
		}
		// Allow API endpoints through (not UI)
		if strings.HasPrefix(path, "/stats") || strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/now") || strings.HasPrefix(path, "/config") || strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/items") || strings.HasPrefix(path, "/img") || strings.HasPrefix(path, "/_next/") {
			return c.Next()
		}
		if c.Locals(userLocalsKey) == nil {
			c.Set("Location", "/login")
			return c.SendStatus(fiber.StatusFound)
		}
		return c.Next()
	}
}

// AdminAccess allows access if either a valid admin session is present or a valid ADMIN_TOKEN is provided.
func AdminAccess(db *sql.DB, adminToken string, cfg config.Config) fiber.Handler {
	base := AdminAuth(adminToken)
	return func(c fiber.Ctx) error {
		// Check session user first
		if u, ok := c.Locals(userLocalsKey).(*userCtx); ok && u != nil && strings.ToLower(u.Role) == "admin" {
			return c.Next()
		}
		// Fallback to legacy header/cookie token check
		return base(c)
	}
}
