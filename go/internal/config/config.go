package config

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

type Config struct {
	EmbyBaseURL     string
	EmbyAPIKey      string
	EmbyExternalURL string
	SQLitePath      string
	WebPath         string

	// Streaming / polling
	KeepAliveSec int
	NowPollSec   int

	// Background sync
	SyncIntervalSec int // e.g. 300 (5 minutes)
	HistoryDays     int // e.g. 2

	// User sync
	UserSyncIntervalSec int `env:"USERSYNC_INTERVAL" envDefault:"43200"` // 12 hours

	// Images
	ImgQuality          int // e.g. 90
	ImgPrimaryMaxWidth  int // e.g. 300
	ImgBackdropMaxWidth int // e.g. 1280

	// Admin refresh
	RefreshChunkSize int // e.g. 200

	// Security
	AdminToken      string // Authentication token for admin endpoints
	WebhookSecret   string // Secret for webhook signature validation
	AdminAutoCookie bool   // If true, server sets HttpOnly cookie to auto-auth UI

	// Logging
	LogLevel  string // DEBUG, INFO, WARN, ERROR
	LogFormat string // json, text, dev
	LogOutput string // stdout, stderr, file path

	// Debug / trace
	NowSseDebug     bool // LOG: /now/stream events
	RefreshSseDebug bool // LOG: /admin/refresh/* SSE
}

func Load() Config {
    dbPath := env("SQLITE_PATH", "/var/lib/emby-analytics/emby.db")
    webPath := env("WEB_PATH", "/app/web")

    // Ensure directories exist; actual DB file creation happens in main preflight
    _ = os.MkdirAll(filepath.Dir(dbPath), 0755)
    _ = os.MkdirAll(webPath, 0755)

	embyBase := env("EMBY_BASE_URL", "http://emby:8096")
	embyKey := env("EMBY_API_KEY", "")
	embyExternal := env("EMBY_EXTERNAL_URL", embyBase)

	cfg := Config{
		EmbyBaseURL:         embyBase,
		EmbyAPIKey:          embyKey,
		EmbyExternalURL:     embyExternal,
		SQLitePath:          dbPath,
		WebPath:             webPath,
		KeepAliveSec:        envInt("KEEPALIVE_SEC", 15),
		NowPollSec:          envInt("NOW_POLL_SEC", 5),
		SyncIntervalSec:     envInt("SYNC_INTERVAL", 300), // Changed from 60 to 300 (5 minutes)
		HistoryDays:         envInt("HISTORY_DAYS", 2),
		ImgQuality:          envInt("IMG_QUALITY", 90),
		ImgPrimaryMaxWidth:  envInt("IMG_PRIMARY_MAX_WIDTH", 300),
		ImgBackdropMaxWidth: envInt("IMG_BACKDROP_MAX_WIDTH", 1280),
		RefreshChunkSize:    envInt("REFRESH_CHUNK_SIZE", 200),
		AdminToken:          env("ADMIN_TOKEN", ""),
		WebhookSecret:       env("WEBHOOK_SECRET", ""),
		AdminAutoCookie:     envBool("ADMIN_AUTO_COOKIE", false),
		LogLevel:            env("LOG_LEVEL", "INFO"),
		LogFormat:           env("LOG_FORMAT", "text"),
		LogOutput:           env("LOG_OUTPUT", "stdout"),
		NowSseDebug:         envBool("NOW_SSE_DEBUG", false),
		RefreshSseDebug:     envBool("REFRESH_SSE_DEBUG", false),
		UserSyncIntervalSec: envInt("USERSYNC_INTERVAL", 43200), // Changed from 3600 to 43200 (12 hours)
	}

	// Auto-generate and persist admin token if not provided
	if cfg.AdminToken == "" {
		tokenFile := filepath.Join(filepath.Dir(dbPath), "admin_token")
		if data, err := os.ReadFile(tokenFile); err == nil {
			t := strings.TrimSpace(string(data))
			if t != "" {
				cfg.AdminToken = t
				fmt.Printf("[INFO] Loaded admin token from %s\n", tokenFile)
			}
		}
		if cfg.AdminToken == "" {
			buf := make([]byte, 32)
			if _, err := rand.Read(buf); err == nil {
				t := hex.EncodeToString(buf)
				if err := os.WriteFile(tokenFile, []byte(t+"\n"), 0600); err == nil {
					cfg.AdminToken = t
					fmt.Printf("[INFO] Generated and saved admin token to %s\n", tokenFile)
				} else {
					fmt.Printf("[WARN] Failed to persist admin token to %s: %v\n", tokenFile, err)
					cfg.AdminToken = t
				}
			} else {
				fmt.Printf("[WARN] Failed to generate admin token: %v\n", err)
			}
		}
		// If token was auto-generated/loaded and cookie opt-in not set, enable it for zero-config UX
		if cfg.AdminToken != "" && !envBool("ADMIN_AUTO_COOKIE", false) {
			cfg.AdminAutoCookie = true
			fmt.Println("[INFO] ADMIN_AUTO_COOKIE enabled automatically (no ADMIN_TOKEN provided).")
		}
	}

    // Default WEBHOOK_SECRET to AdminToken if not provided
    if cfg.WebhookSecret == "" && cfg.AdminToken != "" {
        cfg.WebhookSecret = cfg.AdminToken
        fmt.Println("[INFO] WEBHOOK_SECRET not set; defaulting to ADMIN_TOKEN.")
    }

	fmt.Printf("[INFO] Using SQLite DB at: %s\n", dbPath)
	fmt.Printf("[INFO] Serving static UI from: %s\n", webPath)
	fmt.Printf("[INFO] Emby Base URL: %s\n", embyBase)
	if embyKey == "" {
		fmt.Println("[WARN] EMBY_API_KEY is not set! API calls to Emby will fail.")
	}
	if cfg.AdminToken == "" {
		fmt.Println("[WARN] ADMIN_TOKEN is not set and could not be generated. Admin endpoints will be unprotected.")
	}
	if cfg.AdminAutoCookie && cfg.AdminToken == "" {
		fmt.Println("[WARN] ADMIN_AUTO_COOKIE is true but ADMIN_TOKEN is empty; no cookie will be set.")
	}
	if cfg.WebhookSecret == "" {
		fmt.Println("[WARN] WEBHOOK_SECRET is not set! Webhook endpoint will be unprotected.")
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
