package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/db"
	"emby-analytics/internal/emby"
	admin "emby-analytics/internal/handlers/admin"
	health "emby-analytics/internal/handlers/health"
	images "emby-analytics/internal/handlers/images"
	items "emby-analytics/internal/handlers/items"
	now "emby-analytics/internal/handlers/now"
	stats "emby-analytics/internal/handlers/stats"
	"emby-analytics/internal/tasks"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"
	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

func main() {
	_ = godotenv.Load()

	// ---- Config & Clients ----
	cfg := config.Load()
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)

	// ---- Database Initialization & Migration ----
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer func(dbh *sql.DB) { _ = dbh.Close() }(sqlDB)

	// Run migrations on startup to ensure schema is up-to-date.
	migrationPath := filepath.Join(".", "internal", "db", "migrations")
	if err := db.RunMigrations(sqlDB, migrationPath); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// ---- Real-time Analytics via WebSocket ----
	embyWS := &emby.EmbyWS{
		Cfg: emby.WSConfig{
			BaseURL: cfg.EmbyBaseURL,
			APIKey:  cfg.EmbyAPIKey,
		},
	}
	intervalizer := &tasks.Intervalizer{
		DB:                sqlDB,
		NoProgressTimeout: 90 * time.Second,
		SeekThreshold:     5 * time.Second,
	}
	embyWS.Handler = intervalizer.Handle
	embyWS.Start(context.Background())

	// Background sweeper to clean up timed-out sessions.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			intervalizer.TickTimeoutSweep()
		}
	}()

	// ---- Real-time UI Broadcaster (for Now Playing page) ----
	pollInterval := time.Duration(cfg.NowPollSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	broadcaster := now.NewBroadcaster(em, pollInterval)
	now.SetBroadcaster(broadcaster)
	broadcaster.Start()
	defer broadcaster.Stop()

	// ---- Fiber v3 App ----
	app := fiber.New(fiber.Config{
		EnableIPValidation: true,
		ProxyHeader:        fiber.HeaderXForwardedFor,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// ---- Health Routes ----
	app.Get("/health", health.Health(sqlDB))
	app.Get("/health/emby", health.Emby(em))

	// ---- Stats API Routes ----
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))
	app.Get("/stats/top/users/accurate", stats.AccurateTopUsers(sqlDB))
	app.Get("/stats/top/items", stats.TopItems(sqlDB, em))
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/active-users", stats.ActiveUsersLifetime(sqlDB))
	app.Get("/stats/users/total", stats.UsersTotal(sqlDB))
	app.Get("/stats/user/:id", stats.UserDetailHandler(sqlDB))

	// ---- Item & Image Routes ----
	app.Get("/items/by-ids", items.ByIDs(sqlDB, em))
	imgOpts := images.NewOpts(cfg)
	app.Get("/img/primary/:id", images.Primary(imgOpts))
	app.Get("/img/backdrop/:id", images.Backdrop(imgOpts))

	// ---- Now Playing Routes ----
	app.Get("/now/snapshot", now.Snapshot)
	app.Get("/now/ws", func(c fiber.Ctx) error {
		if ws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}, now.WS())
	app.Post("/now/:id/pause", now.PauseSession)
	app.Post("/now/:id/stop", now.StopSession)
	app.Post("/now/:id/message", now.MessageSession)

	// ---- Admin Routes ----
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
	app.Post("/admin/cleanup/unknown", admin.CleanupUnknownItems(sqlDB, em))
	app.Post("/admin/backfill", admin.BackfillHistory(sqlDB, em))
	app.Get("/admin/debug/history", admin.DebugUserHistory(em))
	app.Get("/admin/debug/recent", admin.DebugUserRecentActivity(em))
	app.Get("/admin/debug/all", admin.DebugUserAllData(em))
	app.All("/admin/fix-pos-units", admin.FixPosUnits(sqlDB))

	// ---- Background Tasks ----
	// The WebSocket listener has replaced the old polling-based sync task.
	// The UserSyncLoop is kept as it populates the lifetime_watch table for "all-time" stats.
	go tasks.StartUserSyncLoop(sqlDB, em, cfg)

	// ---- Static UI Serving ----
	app.Use("/", static.New(cfg.WebPath))

	// SPA Fallback: for any GET request that is not an API/image call, serve the index.html.
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodGet && !startsWithAny(c.Path(), "/health", "/stats", "/admin", "/now", "/img", "/items") {
			return c.SendFile(filepath.Join(cfg.WebPath, "index.html"))
		}
		return c.Next()
	})

	// ---- Start Server ----
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("Starting Emby Analytics server on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatal(err)
	}
}

func startsWithAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
