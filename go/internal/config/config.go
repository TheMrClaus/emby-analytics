package config

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "emby-analytics/internal/media"
)

type Config struct {
	// Legacy single-server fields (backwards compatibility)
	EmbyBaseURL     string
	EmbyAPIKey      string
	EmbyExternalURL string
	
	// Multi-server configuration
	MediaServers      []media.ServerConfig
	DefaultServerID   string
	
	// System paths
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

    // App auth (users + sessions)
    AuthEnabled              bool   // if true, gate UI behind session auth
    AuthRegistrationMode     string // closed|secret|open (default closed)
    AuthRegistrationSecret   string // invite/registration secret when mode=secret
    AuthCookieName           string // cookie name for session token
    AuthSessionTTLMinutes    int    // session lifetime in minutes

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
        AuthEnabled:         envBool("AUTH_ENABLED", true),
        AuthRegistrationMode: env("AUTH_REGISTRATION_MODE", "closed"),
        AuthRegistrationSecret: env("AUTH_REGISTRATION_SECRET", ""),
        AuthCookieName:       env("AUTH_COOKIE_NAME", "ea_session"),
        AuthSessionTTLMinutes: envInt("AUTH_SESSION_TTL_MINUTES", 43200), // 30 days
        LogLevel:            env("LOG_LEVEL", "INFO"),
		LogFormat:           env("LOG_FORMAT", "text"),
		LogOutput:           env("LOG_OUTPUT", "stdout"),
		NowSseDebug:         envBool("NOW_SSE_DEBUG", false),
		RefreshSseDebug:     envBool("REFRESH_SSE_DEBUG", false),
		UserSyncIntervalSec: envInt("USERSYNC_INTERVAL", 43200), // Changed from 3600 to 43200 (12 hours)
	}

	// Load multi-server configuration
	cfg.MediaServers = loadMediaServers(embyBase, embyKey, embyExternal)
	cfg.DefaultServerID = env("DEFAULT_MEDIA_SERVER", getDefaultServerID(cfg.MediaServers))

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

    if cfg.AuthRegistrationMode != "closed" && cfg.AuthRegistrationMode != "open" && cfg.AuthRegistrationMode != "secret" {
        fmt.Println("[WARN] Invalid AUTH_REGISTRATION_MODE; defaulting to 'closed'.")
        cfg.AuthRegistrationMode = "closed"
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

// loadMediaServers loads multi-server configuration with backwards compatibility
func loadMediaServers(legacyEmbyBase, legacyEmbyKey, legacyEmbyExternal string) []media.ServerConfig {
    // 1) Preferred: simple per-type envs EMBY_*, PLEX_*, JELLYFIN_*
    if servers := loadMediaServersSimple(); len(servers) > 0 {
        fmt.Printf("[INFO] Loaded %d media servers from simple per-type env configuration\n", len(servers))
        return servers
    }

    // 2) Backwards-compatible: JSON array in MEDIA_SERVERS
    mediaServersJSON := env("MEDIA_SERVERS", "")
    if mediaServersJSON != "" {
        var servers []media.ServerConfig
        if err := json.Unmarshal([]byte(mediaServersJSON), &servers); err != nil {
            fmt.Printf("[WARN] Failed to parse MEDIA_SERVERS JSON: %v\n", err)
            fmt.Println("[INFO] Falling back to legacy single-server configuration")
        } else {
            fmt.Printf("[INFO] Loaded %d media servers from MEDIA_SERVERS configuration\n", len(servers))
            return servers
        }
    }

    // 3) Optional: numbered env block (still supported for advanced setups)
    if servers := loadMediaServersNumbered(); len(servers) > 0 {
        fmt.Printf("[INFO] Loaded %d media servers from numbered env configuration\n", len(servers))
        return servers
    }

	// Fallback to legacy single-server configuration
	if legacyEmbyKey != "" {
		legacyServer := media.ServerConfig{
			ID:          "default-emby",
			Type:        media.ServerTypeEmby,
			Name:        "Emby Server",
			BaseURL:     legacyEmbyBase,
			APIKey:      legacyEmbyKey,
			ExternalURL: legacyEmbyExternal,
			Enabled:     true,
		}
		fmt.Println("[INFO] Using legacy single-server configuration for Emby")
		return []media.ServerConfig{legacyServer}
	}

	fmt.Println("[WARN] No media servers configured! Set MEDIA_SERVERS or EMBY_API_KEY")
	return []media.ServerConfig{}
}

// loadMediaServersSimple reads EMBY_*, PLEX_*, JELLYFIN_* variables
func loadMediaServersSimple() []media.ServerConfig {
    servers := make([]media.ServerConfig, 0, 3)

    // Emby
    if base := strings.TrimRight(env("EMBY_BASE_URL", ""), "/"); base != "" {
        if key := env("EMBY_API_KEY", ""); key != "" {
            servers = append(servers, media.ServerConfig{
                ID:          "default-emby",
                Type:        media.ServerTypeEmby,
                Name:        env("EMBY_NAME", "Emby"),
                BaseURL:     base,
                APIKey:      key,
                ExternalURL: env("EMBY_EXTERNAL_URL", base),
                Enabled:     envBool("EMBY_ENABLED", true),
            })
        }
    }

    // Plex
    if base := strings.TrimRight(env("PLEX_BASE_URL", ""), "/"); base != "" {
        if key := env("PLEX_API_KEY", ""); key != "" {
            servers = append(servers, media.ServerConfig{
                ID:          "default-plex",
                Type:        media.ServerTypePlex,
                Name:        env("PLEX_NAME", "Plex"),
                BaseURL:     base,
                APIKey:      key,
                ExternalURL: env("PLEX_EXTERNAL_URL", base),
                Enabled:     envBool("PLEX_ENABLED", true),
            })
        }
    }

    // Jellyfin
    if base := strings.TrimRight(env("JELLYFIN_BASE_URL", ""), "/"); base != "" {
        if key := env("JELLYFIN_API_KEY", ""); key != "" {
            servers = append(servers, media.ServerConfig{
                ID:          "default-jellyfin",
                Type:        media.ServerTypeJellyfin,
                Name:        env("JELLYFIN_NAME", "Jellyfin"),
                BaseURL:     base,
                APIKey:      key,
                ExternalURL: env("JELLYFIN_EXTERNAL_URL", base),
                Enabled:     envBool("JELLYFIN_ENABLED", true),
            })
        }
    }

    return servers
}

// loadMediaServersNumbered reads MEDIA_SERVER_1_*, MEDIA_SERVER_2_* ... using MEDIA_SERVERS_COUNT
func loadMediaServersNumbered() []media.ServerConfig {
    cnt := envInt("MEDIA_SERVERS_COUNT", 0)
    if cnt <= 0 {
        return nil
    }
    servers := make([]media.ServerConfig, 0, cnt)
    for i := 1; i <= cnt; i++ {
        prefix := fmt.Sprintf("MEDIA_SERVER_%d_", i)
        t := strings.ToLower(env(prefix+"TYPE", ""))
        if t == "" {
            fmt.Printf("[WARN] %sTYPE missing; skipping server %d\n", prefix, i)
            continue
        }
        // Map to ServerType
        var st media.ServerType
        switch t {
        case string(media.ServerTypeEmby):
            st = media.ServerTypeEmby
        case string(media.ServerTypePlex):
            st = media.ServerTypePlex
        case string(media.ServerTypeJellyfin):
            st = media.ServerTypeJellyfin
        default:
            fmt.Printf("[WARN] %sTYPE unsupported: %s; skipping\n", prefix, t)
            continue
        }

        id := env(prefix+"ID", "")
        if id == "" {
            id = fmt.Sprintf("%s-%d", t, i)
        }
        name := env(prefix+"NAME", "")
        if name == "" {
            name = fmt.Sprintf("%s %d", strings.Title(t), i)
        }
        base := strings.TrimRight(env(prefix+"BASE_URL", ""), "/")
        key := env(prefix+"API_KEY", "")
        ext := env(prefix+"EXTERNAL_URL", base)
        enabled := envBool(prefix+"ENABLED", true)

        if base == "" || key == "" {
            fmt.Printf("[WARN] %sBASE_URL or %sAPI_KEY missing; skipping server '%s'\n", prefix, prefix, id)
            continue
        }

        servers = append(servers, media.ServerConfig{
            ID:          id,
            Type:        st,
            Name:        name,
            BaseURL:     base,
            APIKey:      key,
            ExternalURL: ext,
            Enabled:     enabled,
        })
    }
    return servers
}

// getDefaultServerID returns the first enabled server ID or empty string
func getDefaultServerID(servers []media.ServerConfig) string {
	for _, server := range servers {
		if server.Enabled {
			return server.ID
		}
	}
	return ""
}
