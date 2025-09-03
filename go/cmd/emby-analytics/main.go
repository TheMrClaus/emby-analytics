package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"emby-analytics/internal/config"
	db "emby-analytics/internal/db"
	emby "emby-analytics/internal/emby"
	admin "emby-analytics/internal/handlers/admin"
	configHandler "emby-analytics/internal/handlers/config"
	health "emby-analytics/internal/handlers/health"
	images "emby-analytics/internal/handlers/images"
	items "emby-analytics/internal/handlers/items"
	now "emby-analytics/internal/handlers/now"
	settings "emby-analytics/internal/handlers/settings"
	stats "emby-analytics/internal/handlers/stats"
	"emby-analytics/internal/middleware"
	"emby-analytics/internal/sync"
	tasks "emby-analytics/internal/tasks"

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

	// Open database connection for verification
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("--> FATAL: Failed to open database at %s: %v", cfg.SQLitePath, err)
	}

	// Verify migrations were applied correctly
	var migrationCheck int
	err = sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='play_sessions'`).Scan(&migrationCheck)
	if err != nil || migrationCheck == 0 {
		log.Fatalf("--> FATAL: play_sessions table not found. Migrations failed to apply.")
	}

	// Check for enhanced columns from migration 0005
	var testCol string
	err = sqlDB.QueryRow("SELECT video_method FROM play_sessions LIMIT 1").Scan(&testCol)
	if err != nil {
		log.Println("--> WARNING: Enhanced playback columns not found. Running migration 0005...")
		// The migration should have been applied, but let's ensure it exists
	}

	// Close and reopen connection
	sqlDB.Close()
	sqlDB, err = db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("--> FATAL: Failed to open database at %s: %v", cfg.SQLitePath, err)
	}
	defer func(dbh *sql.DB) { _ = dbh.Close() }(sqlDB)
	log.Println("--> Step 2: Database connection opened successfully.")

	// Initial user sync AFTER schema is ready.
	log.Println("--> Step 3: Performing initial user and lifetime stats sync...")
	tasks.RunUserSyncOnce(sqlDB, em)
	log.Println("--> Initial user sync complete.")

	// ---- Session Processing (Hybrid State-Polling Approach) ----
	sessionProcessor := tasks.NewSessionProcessor(sqlDB)
	log.Println("--> Step 5: Session processor initialized (using playback_reporting approach).")

	pollInterval := time.Duration(cfg.NowPollSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	broadcaster := now.NewBroadcaster(em, pollInterval)
	broadcaster.SessionProcessor = sessionProcessor.ProcessActiveSessions // Connect session processing
	now.SetBroadcaster(broadcaster)
	broadcaster.Start()
	log.Printf("--> Step 6: REST API session polling started (every %v).", pollInterval)
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
	// Optional: auto-auth cookie for UI
	if cfg.AdminAutoCookie && cfg.AdminToken != "" {
		app.Use(func(c fiber.Ctx) error {
			if c.Cookies("admin_token") == "" {
				c.Cookie(&fiber.Cookie{
					Name:     "admin_token",
					Value:    cfg.AdminToken,
					HTTPOnly: true,
					Path:     "/",
				})
			}
			return c.Next()
		})
	}

	// Health Routes
	app.Get("/health", health.Health(sqlDB))
	app.Get("/health/emby", health.Emby(em))
	// Stats API Routes
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))

	app.Get("/stats/top/items", stats.TopItems(sqlDB, em))
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/active-users", stats.ActiveUsersLifetime(sqlDB))
	app.Get("/stats/users/total", stats.UsersTotal(sqlDB))
	app.Get("/stats/user/:id", stats.UserDetailHandler(sqlDB))
	app.Get("/stats/play-methods", stats.PlayMethods(sqlDB, em))
	app.Get("/stats/items/by-codec/:codec", stats.ItemsByCodec(sqlDB))
	app.Get("/stats/items/by-quality/:quality", stats.ItemsByQuality(sqlDB))

	// Backward compatibility routes (hyphenated versions)
	app.Get("/stats/top-users", stats.TopUsers(sqlDB))
	app.Get("/stats/top-items", stats.TopItems(sqlDB, em))
	app.Get("/stats/playback-methods", stats.PlayMethods(sqlDB, em))

	// Configuration Routes
	app.Get("/config", configHandler.GetConfig(cfg))

	// Settings Routes
	app.Get("/settings", settings.GetSettings(sqlDB))
	app.Put("/settings/:key", adminAuth, settings.UpdateSetting(sqlDB))

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
	// Admin Routes with Authentication
	rm := admin.NewRefreshManager()

	// Protected admin endpoints
	adminAuth := middleware.AdminAuth(cfg.AdminToken)
	app.Post("/admin/refresh/start", adminAuth, admin.StartPostHandler(rm, sqlDB, em, cfg.RefreshChunkSize))
	app.Post("/admin/refresh/incremental", adminAuth, admin.StartIncrementalHandler(rm, sqlDB, em))
	app.Get("/admin/refresh/status", adminAuth, admin.StatusHandler(rm))
	app.Get("/admin/webhook/stats", adminAuth, admin.GetWebhookStats())
	app.Post("/admin/reset-all", adminAuth, admin.ResetAllData(sqlDB, em))
	app.Post("/admin/reset-lifetime", adminAuth, admin.ResetLifetimeWatch(sqlDB))
	app.Post("/admin/users/force-sync", adminAuth, admin.ForceUserSync(sqlDB, em))
	app.All("/admin/fix-pos-units", adminAuth, admin.FixPosUnits(sqlDB))
	app.Get("/admin/debug/users", adminAuth, admin.DebugUsers(em))
	app.Post("/admin/recover-intervals", adminAuth, admin.RecoverIntervalsHandler(sqlDB))
	app.Post("/admin/cleanup/intervals/dedupe", adminAuth, admin.CleanupDuplicateIntervals(sqlDB))
	app.Get("/admin/cleanup/intervals/dedupe", adminAuth, admin.CleanupDuplicateIntervals(sqlDB))

	// Debug: inspect recent play_sessions
	app.Get("/admin/debug/sessions", adminAuth, admin.DebugSessions(sqlDB))

	// Backfill playback methods for historical sessions (reason/codec-based)
	app.Post("/admin/cleanup/backfill-playmethods", adminAuth, admin.BackfillPlayMethods(sqlDB))

	// Debug: expose current active sessions from Emby
	app.Get("/admin/debug/emby-sessions", adminAuth, admin.DebugEmbySessions(em))
	// Debug: force-ingest current active Emby sessions into play_sessions
	app.Post("/admin/debug/ingest-active", adminAuth, admin.IngestActiveSessions(sqlDB, em))

	// Webhook endpoint with separate authentication
	webhookAuth := middleware.WebhookAuth(cfg.WebhookSecret)
	app.Post("/admin/webhook/emby", webhookAuth, admin.WebhookHandler(rm, sqlDB, em))

	// Static UI Serving
    app.Use("/", static.New(cfg.WebPath))
    app.Use(func(c fiber.Ctx) error {
        if c.Method() == fiber.MethodGet && !startsWithAny(c.Path(), "/stats", "/health", "/admin", "/now", "/config", "/items", "/img") {
            // If a static exported page exists at /path/index.html, serve it (supports clean URLs without trailing slash)
            reqPath := c.Path()
            // Normalize leading slash
            if !strings.HasPrefix(reqPath, "/") {
                reqPath = "/" + reqPath
            }
            // Try to serve /<path>/index.html
            page := filepath.Join(cfg.WebPath, filepath.FromSlash(reqPath), "index.html")
            if fi, err := os.Stat(page); err == nil && !fi.IsDir() {
                return c.SendFile(page)
            }
            // Fallback to root index.html (for client-side routing if used)
            return c.SendFile(filepath.Join(cfg.WebPath, "index.html"))
        }
        return c.Next()
    })

	// Start sync scheduler
	log.Printf("--> Step 7: Starting smart sync scheduler...")
	scheduler := sync.NewScheduler(sqlDB, em, rm)
	scheduler.Start()

	// Add scheduler stats endpoint (protected)
	app.Get("/admin/scheduler/stats", adminAuth, func(c fiber.Ctx) error {
		stats, err := sync.GetSchedulerStats(sqlDB)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(stats)
	})

	// Start Server
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("--> Step 8: Starting HTTP server on %s", addr)
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
