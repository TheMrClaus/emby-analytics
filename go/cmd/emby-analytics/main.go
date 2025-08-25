package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"

	"emby-analytics/internal/config"
	"emby-analytics/internal/db"
	"emby-analytics/internal/emby"

	// admin
	admin "emby-analytics/internal/handlers/admin"
	// health
	health "emby-analytics/internal/handlers/health"
	// images
	images "emby-analytics/internal/handlers/images"
	// items
	items "emby-analytics/internal/handlers/items"
	// now-playing
	now "emby-analytics/internal/handlers/now"
	// stats
	stats "emby-analytics/internal/handlers/stats"

	// background workers
	"emby-analytics/internal/tasks"

	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

func main() {
	_ = godotenv.Load()

	// ---- config & clients ----
	cfg := config.Load()
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)
	sqlDB, err := db.Open(cfg.SQLitePath)

	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func(dbh *sql.DB) { _ = dbh.Close() }(sqlDB)
	if err := db.EnsureSchema(sqlDB); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	// ---- create broadcaster for now playing ----
	pollInterval := time.Duration(cfg.NowPollSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	broadcaster := now.NewBroadcaster(em, pollInterval)
	now.SetBroadcaster(broadcaster)
	broadcaster.Start()

	// Graceful shutdown for broadcaster
	defer broadcaster.Stop()

	// ---- fiber v3 app ----
	app := fiber.New(fiber.Config{
		EnableIPValidation: true,
		ProxyHeader:        fiber.HeaderXForwardedFor,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// ---- health ----
	app.Get("/health", health.Health(sqlDB))
	app.Get("/health/emby", health.Emby(em))

	// ---- stats API ----
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))
	app.Get("/stats/top/items", stats.TopItems(sqlDB, em))
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/active-users", stats.ActiveUsersLifetime(sqlDB))
	app.Get("/stats/users/total", stats.UsersTotal(sqlDB))
	app.Get("/stats/user/:id", stats.UserDetailHandler(sqlDB))

	app.All("/admin/fix-pos-units", admin.FixPosUnits(db))

	// ---- item helpers ----
	app.Get("/items/by-ids", items.ByIDs(sqlDB, em))

	// ---- images (proxied from Emby) ----
	imgOpts := images.NewOpts(cfg)
	app.Get("/img/primary/:id", images.Primary(imgOpts))
	app.Get("/img/backdrop/:id", images.Backdrop(imgOpts))

	// ---- now playing ----
	// Snapshot: one-shot pull of active sessions
	app.Get("/now/snapshot", now.Snapshot)

	// WebSocket
	app.Get("/now/ws", func(c fiber.Ctx) error {
		if ws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}, now.WS())

	// Controls
	app.Post("/now/:id/pause", now.PauseSession)
	app.Post("/now/:id/stop", now.StopSession)
	app.Post("/now/:id/message", now.MessageSession)

	// ---- admin endpoints (opt-in, keep but unexposed publicly in prod proxies) ----
	rm := admin.NewRefreshManager()
	app.Post("/admin/refresh/start", admin.StartPostHandler(rm, sqlDB, em, cfg.RefreshChunkSize))
	app.Get("/admin/refresh/status", admin.StatusHandler(rm))
	app.Get("/admin/refresh/stream", admin.StreamHandler(rm))
	app.Post("/admin/reset-all", admin.ResetAllData(sqlDB, em))
	app.Post("/admin/reset-lifetime", admin.ResetLifetimeWatch(sqlDB))
	app.Post("/admin/users/force-sync", admin.ForceUserSync(sqlDB, em))
	app.Post("/admin/users/sync", admin.UsersSyncHandler(sqlDB, em, cfg))
	app.Get("/admin/users", admin.ListUsers(sqlDB, em))
	app.Get("/admin/debug/userdata", admin.DebugUserData(em))
	// background loops
	go tasks.StartSyncLoop(sqlDB, em, cfg)
	go tasks.StartUserSyncLoop(sqlDB, em, cfg)

	// ---- static UI (Next.js export in /app/web) ----
	app.Use("/", static.New(cfg.WebPath))

	// SPA fallback for unknown GET routes (but NOT for API)
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodGet && !startsWithAny(c.Path(),
			"/health", "/stats", "/admin", "/now", "/img", "/items") {
			return c.SendFile(cfg.WebPath + "/index.html")
		}
		return fiber.ErrNotFound
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}

func startsWithAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if len(s) >= len(p) && s[:len(p)] == p {
			return true
		}
	}
	return false
}
