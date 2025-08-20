package main

import (
	"log"
	"os"

	// Third-party packages
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"

	// Internal packages
	"emby-analytics/internal/config"
	"emby-analytics/internal/db"
	"emby-analytics/internal/emby"
	"emby-analytics/internal/handlers/admin"
	"emby-analytics/internal/handlers/health"
	"emby-analytics/internal/handlers/images"
	"emby-analytics/internal/handlers/items"
	nown "emby-analytics/internal/handlers/now"
	"emby-analytics/internal/handlers/stats"
	"emby-analytics/internal/tasks"

	// WS middleware (Fiber v3 compatible)
	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

func main() {
	// ==========================================
	// Configuration Setup
	// ==========================================
	_ = godotenv.Load()
	cfg := config.Load()

	// Clients / options
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)
	imgOpts := images.NewOpts(cfg)

	// ==========================================
	// Database Setup
	// ==========================================
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.EnsureSchema(sqlDB); err != nil {
		log.Fatal(err)
	}

	// ==========================================
	// Background Tasks Setup
	// ==========================================
	go tasks.StartSyncLoop(sqlDB, em, cfg)
	go tasks.StartUserSyncLoop(sqlDB, em, cfg)

	// ==========================================
	// Web Server Setup
	// ==========================================
	app := fiber.New()

	// Static UI
	app.Use("/", static.New(cfg.WebPath))

	// ---- WebSocket upgrade gate for /ws/* paths (Fiber v3 + saveblush) ----
	app.Use("/ws", func(c fiber.Ctx) error {
		if ws.IsWebSocketUpgrade(c) {
			// You can stash data into locals to read in the WS handler
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Now-playing routes (snapshot + SSE + WS + controls)
	app.Get("/now", nown.Snapshot)
	app.Get("/now/stream", nown.Stream)
	app.Get("/ws/nowplaying", nown.WS()) // <— WebSocket endpoint

	app.Post("/now/sessions/:id/pause", nown.PauseSession)
	app.Post("/now/sessions/:id/stop", nown.StopSession)
	app.Post("/now/sessions/:id/message", nown.MessageSession)

	// ==========================================
	// API Routes
	// ==========================================
	// Health
	app.Get("/health", health.Health(sqlDB))
	app.Get("/health/emby", health.Emby(em))

	// Stats
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))
	app.Get("/stats/top/items", stats.TopItems(sqlDB))
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/activity", stats.Activity(sqlDB))
	app.Get("/stats/active-users-lifetime", stats.ActiveUsersLifetime(sqlDB))
	app.Get("/stats/users/total", stats.UsersTotal(sqlDB))      // Keep before :id
	app.Get("/stats/users/:id", stats.UserDetailHandler(sqlDB)) // After /total

	// Items
	app.Get("/items/by-ids", items.ByIDs(sqlDB, em))

	// Images
	app.Get("/img/primary/:id", images.Primary(imgOpts))
	app.Get("/img/backdrop/:id", images.Backdrop(imgOpts))

	// Admin refresh (both legacy SSE and FastAPI-compatible endpoints)
	rm := admin.NewRefreshManager()

	// Legacy SSE/GET endpoints (kept)
	app.Get("/admin/refresh/start", admin.StartHandler(rm, sqlDB, em, cfg.RefreshChunkSize))
	app.Get("/admin/refresh/stream", admin.StreamHandler(rm))
	app.Get("/admin/refresh/full", admin.FullHandler(rm, sqlDB, em, cfg.RefreshChunkSize))

	// FastAPI-compatible endpoints used by the UI
	app.Post("/admin/refresh", admin.StartPostHandler(rm, sqlDB, em, cfg.RefreshChunkSize))
	app.Get("/admin/refresh/status", admin.StatusHandler(rm))

	// Users sync trigger
	app.Post("/admin/users/sync", admin.UsersSyncHandler(sqlDB, em, cfg))

	// Admin debug and utility endpoints
	app.Post("/admin/reset-lifetime", admin.ResetLifetimeWatch(sqlDB))
	app.Get("/admin/debug/user-data", admin.DebugUserData(em))
	app.Get("/admin/users", admin.ListUsers(sqlDB, em))
	app.Post("/admin/users/force-sync", admin.ForceUserSync(sqlDB, em))
	app.Post("/admin/cleanup-users", admin.CleanupUsers(sqlDB))
	app.Post("/admin/reset-all-data", admin.ResetAllData(sqlDB, em))

	// ==========================================
	// Start Server
	// ==========================================
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("[INFO] Starting server on :%s", port)
	log.Fatal(app.Listen(":" + port))
}
