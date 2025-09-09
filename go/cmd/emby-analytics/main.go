package main

import (
	"database/sql"
	"fmt"
	"io"
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
	"emby-analytics/internal/logging"
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
	_ = godotenv.Load()
	cfg := config.Load()

	// Initialize structured logging
	var logOutput io.Writer = os.Stdout
	if cfg.LogOutput == "stderr" {
		logOutput = os.Stderr
	} else if cfg.LogOutput != "stdout" && cfg.LogOutput != "" {
		if file, err := os.OpenFile(cfg.LogOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			logOutput = file
			defer file.Close()
		}
	}

	var logLevel logging.Level
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		logLevel = logging.LevelDebug
	case "WARN":
		logLevel = logging.LevelWarn
	case "ERROR":
		logLevel = logging.LevelError
	default:
		logLevel = logging.LevelInfo
	}

	logger := logging.NewLogger(&logging.Config{
		Level:     logLevel,
		Format:    cfg.LogFormat,
		Output:    logOutput,
		AddSource: logLevel == logging.LevelDebug,
	})
	logging.SetDefault(logger)

	logger.Info("=====================================================")
	logger.Info("        Starting Emby Analytics Application")
	logger.Info("=====================================================")
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)

	// ---- Database Initialization & Migration ----
	absPath, err := filepath.Abs(cfg.SQLitePath)
	if err != nil {
		logger.Error("Failed to resolve SQLite path", "error", err, "path", cfg.SQLitePath)
		os.Exit(1)
	}
	dbURL := fmt.Sprintf("sqlite://file:%s?cache=shared&mode=rwc", filepath.ToSlash(absPath))

	if err := db.MigrateUp(dbURL); err != nil {
		logger.Error("Database migrations failed", "error", err, "url", dbURL)
		os.Exit(1)
	}
	logger.Info("Database migrations completed", "path", absPath)

	// Open database connection for verification
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		logger.Error("Failed to open database", "error", err, "path", cfg.SQLitePath)
		os.Exit(1)
	}

	// Verify migrations were applied correctly
	var migrationCheck int
	err = sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='play_sessions'`).Scan(&migrationCheck)
	if err != nil || migrationCheck == 0 {
		logger.Error("play_sessions table not found after migrations", "error", err)
		os.Exit(1)
	}

	// Check for enhanced columns from migration 0005
	var testCol string
	err = sqlDB.QueryRow("SELECT video_method FROM play_sessions LIMIT 1").Scan(&testCol)
	if err != nil {
		logger.Warn("Enhanced playback columns not found, migration 0005 may be needed")
	}

	// Close and reopen connection
	sqlDB.Close()
	sqlDB, err = db.Open(cfg.SQLitePath)
	if err != nil {
		logger.Error("Failed to reopen database", "error", err, "path", cfg.SQLitePath)
		os.Exit(1)
	}
	defer func(dbh *sql.DB) { _ = dbh.Close() }(sqlDB)
	logger.Info("Database connection established")

	// Initial user sync AFTER schema is ready.
	logger.Info("Starting initial user and lifetime stats sync")
	tasks.RunUserSyncOnce(sqlDB, em)
	logger.Info("Initial user sync completed")

	// ---- Session Processing (Hybrid State-Polling Approach) ----
	sessionProcessor := tasks.NewSessionProcessor(sqlDB)
	logger.Info("Session processor initialized")

	pollInterval := time.Duration(cfg.NowPollSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	broadcaster := now.NewBroadcaster(em, pollInterval)
	broadcaster.SessionProcessor = sessionProcessor.ProcessActiveSessions // Connect session processing
	now.SetBroadcaster(broadcaster)
	broadcaster.Start()
	logger.Info("REST API session polling started", "interval", pollInterval)
	defer broadcaster.Stop()

	// ---- Fiber App and Routes ----
	app := fiber.New(fiber.Config{
		EnableIPValidation: true,
		ProxyHeader:        fiber.HeaderXForwardedFor,
	})
	app.Use(recover.New())

	// Add structured logging middleware
	app.Use(logging.FiberMiddleware(logger))

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
	app.Get("/health/frontend", health.FrontendHealth(sqlDB))
	// Stats API Routes
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))

	app.Get("/stats/top/items", stats.TopItems(sqlDB, em))
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/active-users", stats.ActiveUsersLifetime(sqlDB))
	app.Get("/stats/users/total", stats.UsersTotal(sqlDB))
    app.Get("/stats/users/:id", stats.UserDetailHandler(sqlDB, em))
	app.Get("/stats/users/:id/watch-time", stats.UserWatchTimeHandler(sqlDB))
	app.Get("/stats/users/watch-time", stats.AllUsersWatchTimeHandler(sqlDB))
	app.Get("/stats/play-methods", stats.PlayMethods(sqlDB, em))
	app.Get("/stats/items/by-codec/:codec", stats.ItemsByCodec(sqlDB))
	app.Get("/stats/items/by-quality/:quality", stats.ItemsByQuality(sqlDB))
	app.Get("/stats/movies", stats.Movies(sqlDB))
	app.Get("/stats/series", stats.Series(sqlDB))
	app.Get("/stats/top/series", stats.TopSeries(sqlDB))

	// Backward compatibility routes (hyphenated versions)
	app.Get("/stats/top-users", stats.TopUsers(sqlDB))
	app.Get("/stats/top-items", stats.TopItems(sqlDB, em))
	app.Get("/stats/playback-methods", stats.PlayMethods(sqlDB, em))

	// Configuration Routes
	app.Get("/config", configHandler.GetConfig(cfg))

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

	// Settings Routes (admin-protected for updates)
	app.Get("/api/settings", settings.GetSettings(sqlDB))
	app.Put("/api/settings/:key", adminAuth, settings.UpdateSetting(sqlDB))

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
	// Backfill series linkage for episodes
	app.Get("/admin/backfill/series", adminAuth, admin.BackfillSeries(sqlDB, em))
	app.Post("/admin/backfill/series", adminAuth, admin.BackfillSeries(sqlDB, em))
	app.Post("/admin/cleanup/intervals/dedupe", adminAuth, admin.CleanupDuplicateIntervals(sqlDB))
	app.Get("/admin/cleanup/intervals/dedupe", adminAuth, admin.CleanupDuplicateIntervals(sqlDB))
	app.Post("/admin/cleanup/intervals/superset", adminAuth, admin.CleanupSupersetIntervals(sqlDB))
	app.Get("/admin/cleanup/intervals/superset", adminAuth, admin.CleanupSupersetIntervals(sqlDB))
	// Cleanup missing items: scan library_item against Emby and delete safe orphans
	app.Get("/admin/cleanup/missing-items", adminAuth, admin.CleanupMissingItems(sqlDB, em))
	app.Post("/admin/cleanup/missing-items", adminAuth, admin.CleanupMissingItems(sqlDB, em))
	// Cleanup audit logs: view job history and details
	app.Get("/admin/cleanup/jobs", adminAuth, admin.GetCleanupJobs(sqlDB))
	app.Get("/admin/cleanup/jobs/:jobId", adminAuth, admin.GetCleanupJobDetails(sqlDB))
	// Remap stale item_id to a valid destination id
	app.Get("/admin/remap-item", adminAuth, admin.RemapItem(sqlDB, em))
	app.Post("/admin/remap-item", adminAuth, admin.RemapItem(sqlDB, em))
	app.Get("/admin/debug/item-intervals/:id", adminAuth, admin.DebugItemIntervals(sqlDB))

	// Debug: inspect recent play_sessions
	app.Get("/admin/debug/sessions", adminAuth, admin.DebugSessions(sqlDB))

	// Backfill playback methods for historical sessions (reason/codec-based)
	app.Post("/admin/cleanup/backfill-playmethods", adminAuth, admin.BackfillPlayMethods(sqlDB))

	// Admin: backfill started_at from events/intervals
	app.Post("/admin/cleanup/backfill-started-at", adminAuth, admin.BackfillStartedAt(sqlDB))

	// Debug: expose current active sessions from Emby
	app.Get("/admin/debug/emby-sessions", adminAuth, admin.DebugEmbySessions(em))
	// Debug: force-ingest current active Emby sessions into play_sessions
	app.Post("/admin/debug/ingest-active", adminAuth, admin.IngestActiveSessions(sqlDB, em))

	// Debug: resolve Series Id by name
	app.Get("/admin/debug/series-id", adminAuth, admin.DebugFindSeriesID(em))
    // Debug: resolve Series Id from episode id
    app.Get("/admin/debug/series-from-episode", adminAuth, admin.DebugSeriesFromEpisode(em))

	// Admin diagnostics for media metadata coverage
	app.Get("/admin/diagnostics/media-field-coverage", adminAuth, admin.MediaFieldCoverage(sqlDB))
	app.Get("/admin/diagnostics/items/missing", adminAuth, admin.MissingItems(sqlDB))

	// Webhook endpoint with separate authentication
	webhookAuth := middleware.WebhookAuth(cfg.WebhookSecret)
	app.Post("/admin/webhook/emby", webhookAuth, admin.WebhookHandler(rm, sqlDB, em))

	// Static UI Serving
	app.Use("/", static.New(cfg.WebPath))
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == fiber.MethodGet && !startsWithAny(c.Path(), "/stats", "/health", "/admin", "/now", "/config", "/api", "/items", "/img") {
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
	logger.Info("Starting smart sync scheduler")
	scheduler := sync.NewScheduler(sqlDB, em, rm)
	scheduler.Start()

	// Start cleanup scheduler
	logger.Info("Starting cleanup scheduler")
	cleanupScheduler := tasks.NewCleanupScheduler(sqlDB, em, sessionProcessor.Intervalizer)
	cleanupScheduler.Start()

	// Add scheduler stats endpoint (protected)
	app.Get("/admin/scheduler/stats", adminAuth, func(c fiber.Ctx) error {
		stats, err := sync.GetSchedulerStats(sqlDB)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(stats)
	})

	// Add cleanup scheduler stats endpoint (protected)
	app.Get("/admin/cleanup/scheduler/stats", adminAuth, func(c fiber.Ctx) error {
		stats, err := tasks.GetCleanupStats(sqlDB)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(stats)
	})

	// System metrics endpoint (protected)
	app.Get("/admin/metrics", adminAuth, admin.SystemMetricsHandler(sqlDB))

	// Start Server
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	logger.Info("Starting HTTP server", "address", addr)
	if err := app.Listen(addr); err != nil {
		logger.Error("Failed to start HTTP server", "error", err, "address", addr)
		os.Exit(1)
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
