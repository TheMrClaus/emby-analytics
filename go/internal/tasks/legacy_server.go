package tasks

import (
	"strings"

	"emby-analytics/internal/config"
	"emby-analytics/internal/media"
)

// ResolveEmbyServer determines the best-matching Emby server configuration to
// use when legacy single-server paths interact with the multi-server manager.
func ResolveEmbyServer(cfg config.Config, mgr *media.MultiServerManager) (string, media.ServerType) {
	if mgr != nil {
		configs := mgr.GetServerConfigs()
		legacyBase := strings.TrimRight(strings.ToLower(cfg.EmbyBaseURL), "/")
		if legacyBase != "" {
			for id, sc := range configs {
				if sc.Type == media.ServerTypeEmby {
					base := strings.TrimRight(strings.ToLower(sc.BaseURL), "/")
					if base == legacyBase {
						return id, sc.Type
					}
				}
			}
		}
		if def := cfg.DefaultServerID; def != "" {
			if sc, ok := configs[def]; ok && sc.Type == media.ServerTypeEmby {
				return sc.ID, sc.Type
			}
		}
		for id, sc := range configs {
			if sc.Type == media.ServerTypeEmby {
				return id, sc.Type
			}
		}
	}

	for _, sc := range cfg.MediaServers {
		if sc.Type == media.ServerTypeEmby {
			return sc.ID, sc.Type
		}
	}

	return "default-emby", media.ServerTypeEmby
}
