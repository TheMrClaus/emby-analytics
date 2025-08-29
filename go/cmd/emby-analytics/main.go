package main

import (
	"context"
	"database/sql"
	"fmt"
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
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"
	ws "github.com/saveblush/gofiber3-contrib/websocket"
)

// ANSI color helpers
const (
	colReset  = "\033[0m"
	colRed    = "\033[31m"
	colGreen  = "\033[32m"
	colYellow = "\033[33m"
	colCyan   = "\033[36m"
)

func colorStatus(code int) string {
	switch {
	case code >= 200 && code < 300:
		return colGreen
	case code >= 300 && code < 400:
		return colYellow
	case code >= 400:
		return colRed
	default: // 1xx and anything else
		return colCyan
	}
}

func main() {
	log.Println("=====================================================")
	log.Println("        Starting Emby Analytics Application")
	log.Println("=====================================================")

	log.SetFlags(0) // disable default date/time prefix; we print our own timestamp in the middleware

	_ = godotenv.Load()
	cfg := config.Load()
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)

	// ---- Database Initialization & Migration ----
	absPath, err := filepath.Abs(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("--> FATAL: resolving SQLite path: %v", err)
	}
	dbURL := fmt.Sprintf("sqlite://file:%s?cache=shared&mode=rwc", filepath.ToSlash(absPath))

	if err := db.MigrateUp(dbURL); err != nil {
		log.Fatalf("--> FATAL: migrations failed: %v", err)
	}
	log.Println("--> Step 1: Migrations applied (embedded).")

	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("--> FATAL: Failed to open database at %s: %v", cfg.SQLitePath, err)
	}
	defer func(dbh *sql.DB) { _ = dbh.Close() }(sqlDB)
	log.Println("--> Step 2: Database connection opened successfully.")

	// Initial user sync AFTER schema is ready.
	log.Println("--> Step 3: Performing initial user and lifetime stats sync...")
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

	// ---- Background Tasks (WebSocket-first approach) ----
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			intervalizer.TickTimeoutSweep()
		}
	}()
	log.Println("--> Step 6: Background tasks initiated (sync loop deprecated, using WebSocket-only approach).")

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

	// Colored single-line logger (status-based)
	app.Use(func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		// After response
		status := c.Response().StatusCode()
		latency := time.Since(start)
		ip := c.IP()
		method := c.Method()
		path := c.Path()
		ts := time.Now().Format("15:04:05")

		// Colorize just the status code by class (2xx green, 3xx yellow, 4xx/5xx red, 1xx cyan)
		statusColor := colorStatus(status)

		log.Printf("%s | %s%d%s | %v | %s | %s | %s",
			ts, statusColor, status, colReset, latency, ip, method, path)

		return err
	})

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
	app.Get("/stats/play-methods", stats.PlayMethods(sqlDB))
	app.Get("/stats/items/by-codec/:codec", stats.ItemsByCodec(sqlDB))
	app.Get("/stats/items/by-quality/:quality", stats.ItemsByQuality(sqlDB))

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
	app.Get("/admin/debug/users", admin.DebugUsers(em))

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
