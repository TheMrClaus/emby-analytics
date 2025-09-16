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
    auth "emby-analytics/internal/handlers/auth"
    configHandler "emby-analytics/internal/handlers/config"
    verhandler "emby-analytics/internal/handlers/version"
    health "emby-analytics/internal/handlers/health"
    images "emby-analytics/internal/handlers/images"
    items "emby-analytics/internal/handlers/items"
    now "emby-analytics/internal/handlers/now"
    serversHandler "emby-analytics/internal/handlers/servers"
    settings "emby-analytics/internal/handlers/settings"
    stats "emby-analytics/internal/handlers/stats"
    "emby-analytics/internal/logging"
    "emby-analytics/internal/middleware"
    "emby-analytics/internal/monitors"
    "emby-analytics/internal/sync"
    tasks "emby-analytics/internal/tasks"

    // Multi-server clients
    "emby-analytics/internal/media"
    "emby-analytics/internal/plex"
    "emby-analytics/internal/jellyfin"

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

    // Build MultiServerManager (Plex/Jellyfin for now; Emby support via legacy paths)
    multiMgr := media.NewMultiServerManager()
    for _, sc := range cfg.MediaServers {
        switch sc.Type {
        case media.ServerTypePlex:
            multiMgr.AddServer(sc, plex.New(sc))
        case media.ServerTypeJellyfin:
            multiMgr.AddServer(sc, jellyfin.New(sc))
        case media.ServerTypeEmby:
            multiMgr.AddServer(sc, media.NewEmbyAdapter(sc))
        }
    }

    // ---- Database Initialization & Migration ----
    absPath, err := filepath.Abs(cfg.SQLitePath)
    if err != nil {
        logger.Error("Failed to resolve SQLite path", "error", err, "path", cfg.SQLitePath)
        os.Exit(1)
    }
    // Ensure DB directory exists and DB file is present (created as current docker user 1000:1000)
    dbDir := filepath.Dir(absPath)
    if err := os.MkdirAll(dbDir, 0755); err != nil {
        logger.Error("Failed to create database directory", "error", err, "dir", dbDir)
        os.Exit(1)
    }
    // Create DB file if missing, and verify read-write access
    if f, err := os.OpenFile(absPath, os.O_RDWR|os.O_CREATE, 0644); err != nil {
        logger.Error("Failed to create/open SQLite file", "error", err, "path", absPath)
        logger.Error("Check that the directory is writable by UID:GID 1000:1000 or adjust host bind mount permissions")
        os.Exit(1)
    } else {
        _ = f.Close()
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

	// Ensure auth tables exist even if a prior image failed to apply late migrations
	ensureAuthTables(sqlDB, logger)

	// Ensure genres column exists (migration 0013) for legacy DBs that got stuck at v12
	ensureGenresColumn(sqlDB, logger)

	// If we detect all late schema pieces present but migration version < 14, bump it to avoid reattempts
	bumpLegacyMigrationVersion(sqlDB, logger)

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
    sessionProcessor := tasks.NewSessionProcessor(sqlDB, multiMgr)
	logger.Info("Session processor initialized")

	pollInterval := time.Duration(cfg.NowPollSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
    broadcaster := now.NewBroadcaster(em, pollInterval)
    broadcaster.SessionProcessor = sessionProcessor.ProcessActiveSessions
    now.SetBroadcaster(broadcaster)
    now.SetMultiServerManager(multiMgr)
    serversHandler.SetManager(multiMgr)
	broadcaster.Start()
	logger.Info("REST API session polling started", "interval", pollInterval)
	defer broadcaster.Stop()

	// ---- Fiber App and Routes ----
    app := fiber.New(fiber.Config{
        EnableIPValidation: true,
        ProxyHeader:        fiber.HeaderXForwardedFor,
    })
    app.Use(recover.New())

    // CORS with credentials support (echo Origin)
    app.Use(func(c fiber.Ctx) error {
        origin := c.Get("Origin")
        if origin != "" {
            c.Set("Access-Control-Allow-Origin", origin)
            c.Set("Vary", "Origin")
            c.Set("Access-Control-Allow-Credentials", "true")
            c.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Token")
            c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
            if c.Method() == fiber.MethodOptions {
                return c.SendStatus(fiber.StatusNoContent)
            }
        }
        return c.Next()
    })

    // Add structured logging middleware
    app.Use(logging.FiberMiddleware(logger))

    // Attach session user to context
    app.Use(middleware.AttachUser(sqlDB, cfg))

	// Health Routes
	// Optional: auto-auth cookie for UI
    if cfg.AdminAutoCookie && cfg.AdminToken != "" {
        app.Use(func(c fiber.Ctx) error {
            c.Cookie(&fiber.Cookie{
                Name:     "admin_token",
                Value:    cfg.AdminToken,
                HTTPOnly: true,
                Path:     "/",
            })
            return c.Next()
        })
    }

	// Health Routes
	app.Get("/health", health.Health(sqlDB))
	app.Get("/health/emby", health.Emby(em))
	app.Get("/health/frontend", health.FrontendHealth(sqlDB))
	// Version Route
	app.Get("/version", verhandler.GetVersion())
	// Stats API Routes
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))

    app.Get("/stats/top/items", stats.TopItems(sqlDB, em))
    // Inject manager so TopItems can enrich non-Emby items
    stats.SetMultiServerManager(multiMgr)
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/active-users", stats.ActiveUsersLifetime(sqlDB))
	app.Get("/stats/users/total", stats.UsersTotal(sqlDB))
    app.Get("/stats/users/:id", stats.UserDetailHandler(sqlDB, em))
	app.Get("/stats/users/:id/watch-time", stats.UserWatchTimeHandler(sqlDB))
	app.Get("/stats/users/watch-time", stats.AllUsersWatchTimeHandler(sqlDB))
	app.Get("/stats/play-methods", stats.PlayMethods(sqlDB, em))
    app.Get("/stats/items/by-codec/:codec", stats.ItemsByCodec(sqlDB))
    app.Get("/stats/items/by-genre/:genre", stats.ItemsByGenre(sqlDB))
    app.Get("/stats/series/by-genre/:genre", stats.SeriesByGenre(sqlDB))
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
    // Multi-server-aware items lookup (falls back to legacy where needed)
    app.Get("/items/by-ids", items.ByIDsMS(sqlDB, multiMgr))
	imgOpts := images.NewOpts(cfg)
	app.Get("/img/primary/:id", images.Primary(imgOpts))
	app.Get("/img/backdrop/:id", images.Backdrop(imgOpts))
	// Multi-server image routes
	app.Get("/img/primary/:server/:id", images.MultiServerPrimary(multiMgr))
	// Now Playing Routes
    app.Get("/api/now-playing/summary", now.Summary)
    // Legacy single-Emby snapshot remains for compatibility with current UI
    app.Get("/now/snapshot", now.Snapshot)
    // New multi-server snapshot for updated UI/clients
    app.Get("/api/now/snapshot", now.MultiSnapshot)
    // Multi-server WebSocket stream (optional ?server=emby|plex|jellyfin|all)
    app.Get("/api/now/ws", func(c fiber.Ctx) error {
        if ws.IsWebSocketUpgrade(c) { return c.Next() }
        return fiber.ErrUpgradeRequired
    }, ws.New(now.MultiWS()))
	app.Get("/now/ws", func(c fiber.Ctx) error {
		if ws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}, now.WS())
	app.Post("/now/:id/pause", now.PauseSession)
	app.Post("/now/:id/stop", now.StopSession)
	app.Post("/now/:id/message", now.MessageSession)
    // Server list/health
    app.Get("/api/servers", serversHandler.List())

    // Server-aware now controls
    app.Post("/api/now/sessions/:server/:id/pause", now.MultiPauseSession)
    app.Post("/api/now/sessions/:server/:id/stop", now.MultiStopSession)
    app.Post("/api/now/sessions/:server/:id/message", now.MultiMessageSession)

    // Admin Routes with Authentication
	rm := admin.NewRefreshManager()

    // Protected admin endpoints (admin session OR ADMIN_TOKEN)
    adminAuth := middleware.AdminAccess(sqlDB, cfg.AdminToken, cfg)

	// Settings Routes (admin-protected for updates)
	app.Get("/api/settings", settings.GetSettings(sqlDB))
	app.Put("/api/settings/:key", adminAuth, settings.UpdateSetting(sqlDB))

	app.Post("/admin/refresh/start", adminAuth, admin.StartPostHandler(rm, sqlDB, em, cfg.RefreshChunkSize))
    app.Post("/admin/refresh/incremental", adminAuth, admin.StartIncrementalHandler(rm, sqlDB, em))
    app.Post("/admin/enrich/missing-items", adminAuth, admin.EnrichMissingItems(sqlDB, multiMgr))
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
    // Fix legacy fallback intervals that over-count paused time
    app.Post("/admin/cleanup/intervals/fix-fallback", adminAuth, admin.FixFallbackIntervals(sqlDB))
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
    // Enrich missing usernames for Plex/Jellyfin sessions
    app.Post("/admin/enrich/user-names", adminAuth, admin.EnrichUserNames(sqlDB, multiMgr))

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

    // Auth endpoints
    app.Post("/auth/login", auth.LoginHandler(sqlDB, cfg))
    app.Post("/auth/logout", auth.LogoutHandler(sqlDB, cfg))
    app.Post("/auth/register", auth.RegisterHandler(sqlDB, cfg))
    app.Get("/auth/me", auth.MeHandler(sqlDB, cfg))
    app.Get("/auth/config", auth.ConfigHandler(sqlDB, cfg))

    // Static UI Serving
    if cfg.AuthEnabled {
        app.Use(middleware.RequireUserForUI(cfg))
    }
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

	// Start 4K video transcoding monitor
	logger.Info("Starting 4K video transcoding monitor")
	transcodingMonitor := monitors.NewTranscodingMonitor(sqlDB, em, 30*time.Second)
	transcodingMonitor.Start()
	defer transcodingMonitor.Stop()

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

    // App user management (admin-only)
    app.Get("/admin/app-users", adminAuth, auth.ListAppUsers(sqlDB))
    app.Post("/admin/app-users", adminAuth, auth.CreateAppUser(sqlDB))
    app.Put("/admin/app-users/:id", adminAuth, auth.UpdateAppUser(sqlDB))
    app.Delete("/admin/app-users/:id", adminAuth, auth.DeleteAppUser(sqlDB))

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

// ensureAuthTables defensively creates auth tables if they are missing.
func ensureAuthTables(db *sql.DB, logger logging.Logger) {
    // app_user
    if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS app_user (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT NOT NULL UNIQUE,
        password_hash TEXT NOT NULL,
        role TEXT NOT NULL DEFAULT 'user',
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );`); err != nil {
        logger.Warn("Failed to ensure app_user table", "error", err)
    } else {
        logger.Info("Auth: ensured app_user table")
    }
    // app_session
    if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS app_session (
        token TEXT PRIMARY KEY,
        user_id INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        expires_at TIMESTAMP NOT NULL
    );`); err != nil {
        logger.Warn("Failed to ensure app_session table", "error", err)
    } else {
        logger.Info("Auth: ensured app_session table")
    }
    // indexes
    if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_app_session_user ON app_session(user_id);`); err != nil {
        logger.Warn("Failed to ensure idx_app_session_user", "error", err)
    }
    if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_app_user_username ON app_user(username);`); err != nil {
        logger.Warn("Failed to ensure idx_app_user_username", "error", err)
    }
}

func ensureGenresColumn(db *sql.DB, logger logging.Logger) {
    // Check if column exists
    rows, err := db.Query(`PRAGMA table_info(library_item);`)
    if err != nil {
        logger.Warn("Failed to inspect library_item schema", "error", err)
        return
    }
    defer rows.Close()
    has := false
    for rows.Next() {
        var cid int
        var name, ctype string
        var notnull int
        var dflt interface{}
        var pk int
        _ = rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
        if strings.EqualFold(name, "genres") {
            has = true
            break
        }
    }
    if !has {
        if _, err := db.Exec(`ALTER TABLE library_item ADD COLUMN genres TEXT;`); err != nil {
            logger.Warn("Failed to add genres column (may already exist)", "error", err)
        } else {
            logger.Info("Auth: ensured library_item.genres column")
        }
    }
}

func bumpLegacyMigrationVersion(db *sql.DB, logger logging.Logger) {
    // Determine if app_user and app_session exist
    var cnt int
    _ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='app_user'`).Scan(&cnt)
    if cnt == 0 {
        return
    }
    _ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='app_session'`).Scan(&cnt)
    if cnt == 0 {
        return
    }
    // Check migration version
    var version int
    var dirty int
    if err := db.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
        return
    }
    if version < 14 {
        if _, err := db.Exec(`UPDATE schema_migrations SET version=14, dirty=0`); err == nil {
            logger.Info("Auth: bumped schema_migrations version to 14 to reflect ensured tables")
        }
    }
}
