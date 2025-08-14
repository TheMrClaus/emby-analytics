package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	EmbyBaseURL  string
	EmbyAPIKey   string
	SQLitePath   string
	KeepAliveSec int
}

func Load() Config {
	path := env("SQLITE_PATH", "/var/lib/emby-analytics/emby.db")
	// Ensure parent dir exists
	os.MkdirAll(filepath.Dir(path), 0755)

	return Config{
		EmbyBaseURL:  env("EMBY_BASE_URL", "http://emby:8096"),
		EmbyAPIKey:   env("EMBY_API_KEY", ""),
		SQLitePath:   path,
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
