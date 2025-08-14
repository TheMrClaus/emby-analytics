package main

import (
	"bufio"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"emby-analytics/internal/config"
	"emby-analytics/internal/db"
	"emby-analytics/internal/handlers/health"
	"emby-analytics/internal/handlers/stats"
)

func main() {
	cfg := config.Load()

	sqlDB, err := db.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.EnsureSchema(sqlDB); err != nil {
		log.Fatal(err)
	}

	app := fiber.New()

	// Static UI
	app.Use("/", static.New("./go/web"))

	// Health routes
	app.Get("/health", health.Health(sqlDB))

	// API routes
	app.Get("/stats/overview", stats.Overview(sqlDB))

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(app.Listen(":" + port))
}
