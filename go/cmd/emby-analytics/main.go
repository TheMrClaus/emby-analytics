package main

import (
	"bufio"
	"log"
	"os"
	"time"

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
	"emby-analytics/internal/handlers/stats"
)

func main() {
	// ==========================================
	// Configuration Setup
	// ==========================================

	// Load .env file if it exists (for binary users)
	_ = godotenv.Load()

	// Load application configuration
	cfg := config.Load()

	// Initialize Emby client
	em := emby.New(cfg.EmbyBaseURL, cfg.EmbyAPIKey)

	// Configure image handling options
	imgOpts := images.NewOpts(cfg)

	// ==========================================
	// Database Setup
	// ==========================================

	// Open SQLite database connection
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatal(err)
	}

	// Ensure database schema is up to date
	if err := db.EnsureSchema(sqlDB); err != nil {
		log.Fatal(err)
	}

	// ==========================================
	// Web Server Setup
	// ==========================================

	// Create Fiber application instance
	app := fiber.New()

	// Serve static UI files from configured web path
	app.Use("/", static.New(cfg.WebPath))

	// ==========================================
	// API Routes
	// ==========================================

	// Health check endpoint
	app.Get("/health", health.Health(sqlDB))

	// Statistics endpoints
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))
	app.Get("/stats/top/items", stats.TopItems(sqlDB))
	app.Get("/stats/qualities", stats.Qualities(sqlDB))
	app.Get("/stats/codecs", stats.Codecs(sqlDB))
	app.Get("/stats/activity", stats.Activity(sqlDB))
	app.Get("/stats/users/:id", stats.UserDetailHandler(sqlDB))

	// Item endpoints
	app.Get("/items/by-ids", items.ByIDs(sqlDB, em))

	// Image proxy endpoints
	app.Get("/img/primary/:id", images.Primary(imgOpts))
	app.Get("/img/backdrop/:id", images.Backdrop(imgOpts))

	// Admin endpoints
	rm := admin.NewRefreshManager()
	app.Get("/admin/refresh/start", admin.StartHandler(rm, sqlDB, em))
	app.Get("/admin/refresh/stream", admin.StreamHandler(rm))

	// ==========================================
	// Server-Sent Events (SSE) Keepalive
	// ==========================================

	app.Get("/now/stream", func(c fiber.Ctx) error {
		// Set SSE headers
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			// Send initial hello event
			w.WriteString("event: hello\ndata: {}\n\n")
			_ = w.Flush()

			// Set up periodic keepalive ticker
			t := time.NewTicker(time.Duration(cfg.KeepAliveSec) * time.Second)
			defer t.Stop()

			// Send keepalive events at configured intervals
			for range t.C {
				w.WriteString("event: keepalive\ndata: {}\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		})
	})

	// ==========================================
	// Start Server
	// ==========================================

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("[INFO] Starting server on :%s", port)
	log.Fatal(app.Listen(":" + port))
}
