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
	log.Println("=====================================================")
	log.Println("        Starting Emby Analytics Application")
	log.Println("=====================================================")

	_ = godotenv.Load()
	cfg := config.Load()
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)

	// ---- Database Initialization & Migration ----
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("--> FATAL: Failed to open database at %s: %v", cfg.SQLitePath, err)
	}
	defer func(dbh *sql.DB) { _ = dbh.Close() }(sqlDB)
	log.Println("--> Step 1: Database connection opened successfully.")

	// 1. Unconditionally create the base tables required for the app to start.
	if err := db.EnsureBaseSchema(sqlDB); err != nil {
		log.Fatalf("--> FATAL: Failed to ensure base schema: %v", err)
	}
	log.Println("--> Step 2: Base schema (user, item, lifetime) ensured.")

	// 2. Run the full migrations to create analytics tables.
	migrationPath := filepath.Join(".", "internal", "db", "migrations")
	if err := db.RunMigrationsWithLogging(sqlDB, migrationPath, log.Default()); err != nil {
		log.Fatalf("--> FATAL: Failed to run migrations: %v", err)
	}
	log.Println("--> Step 3: Analytics schema (sessions, intervals) migrated.")

	// 3. Perform the initial user sync synchronously AFTER the schema is ready.
	log.Println("--> Step 4: Performing initial user and lifetime stats sync...")
	tasks.RunUserSyncOnce(sqlDB, em)
	log.Println("--> Initial user sync complete.")

	// ---- Real-time Analytics via WebSocket ----
	embyWS := &emby.EmbyWS{
		Cfg: emby.WSConfig{BaseURL: cfg.EmbyBaseURL, APIKey: cfg.EmbyAPIKey},
	}
	intervalizer := &tasks.Intervalizer{
		DB:                sqlDB,
		NoProgressTimeout: 90 * time.Second,
		SeekThreshold:     5 * time.Second,
	}
	embyWS.Handler = intervalizer.Handle
	embyWS.Start(context.Background())
	log.Println("--> Step 5: Emby WebSocket listener started.")

	// ---- Background Tasks (Now started AFTER initial sync) ----
	go tasks.StartUserSyncLoop(sqlDB, em, cfg)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			intervalizer.TickTimeoutSweep()
		}
	}()
	log.Println("--> Step 6: Background tasks initiated.")

	pollInterval := time.Duration(cfg.NowPollSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	broadcaster := now.NewBroadcaster(em, pollInterval)
	now.SetBroadcaster(broadcaster)
	broadcaster.Start()
	defer broadcaster.Stop()

	// ---- Fiber App and Routes ----
	app := fiber.New(fiber.Config{
		EnableIPValidation: true,
		ProxyHeader:        fiber.HeaderXForwardedFor,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// Health Routes
	app.Get("/health", health.Health(sqlDB))
	app.Get("/health/emby", health.Emby(em))
	// Stats API Routes
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
	// Item & Image Routes
	app.Get("/items/by-ids", items.ByIDs(sqlDB, em))
	imgOpts := images.NewOpts(cfg)
	app.Get("/img/primary/:id", images.Primary(imgOpts))
	app.Get("/img/backdrop/:id", images.Backdrop(imgOpts))
	// Now Playing Routes
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
	// Admin Routes
	rm := admin.NewRefreshManager()
	app.Post("/admin/refresh/start", admin.StartPostHandler(rm, sqlDB, em, cfg.RefreshChunkSize))
	app.Get("/admin/refresh/status", admin.StatusHandler(rm))
	app.Post("/admin/reset-all", admin.ResetAllData(sqlDB, em))
	app.Post("/admin/reset-lifetime", admin.ResetLifetimeWatch(sqlDB))
	app.Post("/admin/users/force-sync", admin.ForceUserSync(sqlDB, em))
	app.All("/admin/fix-pos-units", admin.FixPosUnits(sqlDB))

	// Static UI Serving
	app.Use("/", static.New(cfg.WebPath))
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodGet && !strings.HasPrefix(c.Path(), "/api") {
			return c.SendFile(filepath.Join(cfg.WebPath, "index.html"))
		}
		return c.Next()
	})

	// Start Server
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("--> Step 7: Starting HTTP server on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("--> FATAL: Failed to start server: %v", err)
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
