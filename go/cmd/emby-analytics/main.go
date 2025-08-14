package main

import (
	"bufio"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"

	"emby-analytics/internal/config"
	"emby-analytics/internal/db"
	"emby-analytics/internal/handlers/health"
	"emby-analytics/internal/handlers/stats"
)

func main() {
	// Load .env file if it exists (for binary users)
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Init DB
	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.EnsureSchema(sqlDB); err != nil {
		log.Fatal(err)
	}

	app := fiber.New()

	// Serve static UI from WebPath
	app.Use("/", static.New(cfg.WebPath))

	// Routes
	app.Get("/health", health.Health(sqlDB))
	app.Get("/stats/overview", stats.Overview(sqlDB))
	app.Get("/stats/usage", stats.Usage(sqlDB))
	app.Get("/stats/top/users", stats.TopUsers(sqlDB))
	app.Get("/stats/top/items", stats.TopItems(sqlDB))

	// SSE keepalive
	app.Get("/now/stream", func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		return c.SendStreamWriter(func(w *bufio.Writer) {
			w.WriteString("event: hello\ndata: {}\n\n")
			_ = w.Flush()
			t := time.NewTicker(time.Duration(cfg.KeepAliveSec) * time.Second)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					w.WriteString("event: keepalive\ndata: {}\n\n")
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("[INFO] Starting server on :%s", port)
	log.Fatal(app.Listen(":" + port))
}
