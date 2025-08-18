package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	EmbyBaseURL string
	EmbyAPIKey  string
	SQLitePath  string
	WebPath     string

	// Streaming / polling
	KeepAliveSec int
	NowPollSec   int

	// Background sync
	SyncIntervalSec int // e.g. 60
	HistoryDays     int // e.g. 2

	// User sync
	UserSyncIntervalSec int `env:"USERSYNC_INTERVAL" envDefault:"3600"` // 1 hour

	// Images
	ImgQuality          int // e.g. 90
	ImgPrimaryMaxWidth  int // e.g. 300
	ImgBackdropMaxWidth int // e.g. 1280

	// Admin refresh
	RefreshChunkSize int // e.g. 200

	// Debug / trace
	NowSseDebug     bool // LOG: /now/stream events
	RefreshSseDebug bool // LOG: /admin/refresh/* SSE
}

func Load() Config {
	dbPath := env("SQLITE_PATH", "/var/lib/emby-analytics/emby.db")
	webPath := env("WEB_PATH", "/app/web")

	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)
	_ = os.MkdirAll(webPath, 0755)

	embyBase := env("EMBY_BASE_URL", "http://emby:8096")
	embyKey := env("EMBY_API_KEY", "")

	cfg := Config{
		EmbyBaseURL:         embyBase,
		EmbyAPIKey:          embyKey,
		SQLitePath:          dbPath,
		WebPath:             webPath,
		KeepAliveSec:        envInt("KEEPALIVE_SEC", 15),
		NowPollSec:          envInt("NOW_POLL_SEC", 5),
		SyncIntervalSec:     envInt("SYNC_INTERVAL", 60),
		HistoryDays:         envInt("HISTORY_DAYS", 2),
		ImgQuality:          envInt("IMG_QUALITY", 90),
		ImgPrimaryMaxWidth:  envInt("IMG_PRIMARY_MAX_WIDTH", 300),
		ImgBackdropMaxWidth: envInt("IMG_BACKDROP_MAX_WIDTH", 1280),
		RefreshChunkSize:    envInt("REFRESH_CHUNK_SIZE", 200),
		NowSseDebug:         envBool("NOW_SSE_DEBUG", false),
		RefreshSseDebug:     envBool("REFRESH_SSE_DEBUG", false),
		UserSyncIntervalSec: envInt("USERSYNC_INTERVAL", 3600),
	}

	fmt.Printf("[INFO] Using SQLite DB at: %s\n", dbPath)
	fmt.Printf("[INFO] Serving static UI from: %s\n", webPath)
	fmt.Printf("[INFO] Emby Base URL: %s\n", embyBase)
	if embyKey == "" {
		fmt.Println("[WARN] EMBY_API_KEY is not set! API calls to Emby will fail.")
	}
	return cfg
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

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
