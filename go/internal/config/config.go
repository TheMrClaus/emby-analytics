package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	EmbyBaseURL  string
	EmbyAPIKey   string
	SQLitePath   string
	WebPath      string
	KeepAliveSec int
}

func Load() Config {
	dbPath := env("SQLITE_PATH", "/var/lib/emby-analytics/emby.db")
	webPath := env("WEB_PATH", "/app/web")

	// Ensure DB folder exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		fmt.Printf("[WARN] Could not create DB directory: %v\n", err)
	}

	// Ensure static UI folder exists
	if err := os.MkdirAll(webPath, 0755); err != nil {
		fmt.Printf("[WARN] Could not create web directory: %v\n", err)
	}

	embyBase := env("EMBY_BASE_URL", "http://emby:8096")
	embyKey := env("EMBY_API_KEY", "")

	// Startup info
	fmt.Printf("[INFO] Using SQLite DB at: %s\n", dbPath)
	fmt.Printf("[INFO] Serving static UI from: %s\n", webPath)
	fmt.Printf("[INFO] Emby Base URL: %s\n", embyBase)

	// Warn if API key missing
	if embyKey == "" {
		fmt.Println("[WARN] EMBY_API_KEY is not set! API calls to Emby will fail.")
	}

	return Config{
		EmbyBaseURL:  embyBase,
		EmbyAPIKey:   embyKey,
		SQLitePath:   dbPath,
		WebPath:      webPath,
		KeepAliveSec: envInt("KEEPALIVE_SEC", 15),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
