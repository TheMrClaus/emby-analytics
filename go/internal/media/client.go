package media

import (
	"context"
	"sync"
	"time"

	"emby-analytics/internal/sessioncache"
)

// MediaServerClient defines the unified interface for all media server types
// This interface abstracts common operations across Emby, Plex, and Jellyfin servers
type MediaServerClient interface {
	// Server identification
	GetServerID() string
	GetServerType() ServerType
	GetServerName() string

	// Core functionality
	GetActiveSessions() ([]Session, error)
	GetSystemInfo() (*SystemInfo, error)
	GetUsers() ([]User, error)
	GetUserData(userID string) ([]UserDataItem, error)

	// Media item operations
	ItemsByIDs(ids []string) ([]MediaItem, error)
	GetUserPlayHistory(userID string, daysBack int) ([]PlayHistoryItem, error)

	// Session control operations
	PauseSession(sessionID string) error
	UnpauseSession(sessionID string) error
	StopSession(sessionID string) error
	SendMessage(sessionID, header, text string, timeoutMs int) error

	// Health check
	CheckHealth() (*ServerHealth, error)
}

// ClientFactory creates MediaServerClient instances based on server configuration
type ClientFactory interface {
	CreateClient(config ServerConfig) (MediaServerClient, error)
}

// MultiServerManager manages multiple media servers
type MultiServerManager struct {
	clients map[string]MediaServerClient
	configs map[string]ServerConfig
	cache   *sessioncache.SessionCache
}

// NewMultiServerManager creates a new multi-server manager
func NewMultiServerManager(cache *sessioncache.SessionCache) *MultiServerManager {
	return &MultiServerManager{
		clients: make(map[string]MediaServerClient),
		configs: make(map[string]ServerConfig),
		cache:   cache,
	}
}

// AddServer adds a server to the manager
func (m *MultiServerManager) AddServer(config ServerConfig, client MediaServerClient) {
	m.configs[config.ID] = config
	m.clients[config.ID] = client
}

// RemoveServer removes a server from the manager
func (m *MultiServerManager) RemoveServer(serverID string) {
	delete(m.configs, serverID)
	delete(m.clients, serverID)
}

// GetClient returns a client for the specified server ID
func (m *MultiServerManager) GetClient(serverID string) (MediaServerClient, bool) {
	client, exists := m.clients[serverID]
	return client, exists
}

// GetAllClients returns all registered clients
func (m *MultiServerManager) GetAllClients() map[string]MediaServerClient {
	return m.clients
}

// ClientsByType returns enabled clients matching a given server type
func (m *MultiServerManager) ClientsByType(t ServerType) []MediaServerClient {
	out := []MediaServerClient{}
	for id, client := range m.clients {
		if client == nil {
			continue
		}
		cfg, ok := m.configs[id]
		if !ok || !cfg.Enabled {
			continue
		}
		if cfg.Type == t {
			out = append(out, client)
		}
	}
	return out
}

// GetEnabledClients returns only enabled clients
func (m *MultiServerManager) GetEnabledClients() map[string]MediaServerClient {
	enabled := make(map[string]MediaServerClient)
	for serverID, client := range m.clients {
		if client == nil {
			continue
		}
		if config, exists := m.configs[serverID]; exists && config.Enabled {
			enabled[serverID] = client
		}
	}
	return enabled
}

// GetAllSessions aggregates sessions from all enabled servers
func (m *MultiServerManager) GetAllSessions() ([]Session, error) {
	var allSessions []Session

	for _, client := range m.GetEnabledClients() {
		if client == nil {
			continue
		}
		sessions, err := client.GetActiveSessions()
		if err != nil {
			// Log error but continue with other servers
			continue
		}
		allSessions = append(allSessions, sessions...)
	}

	return allSessions, nil
}

// GetAllSessionsCached returns sessions from cache if fresh, otherwise fetches from servers
func (m *MultiServerManager) GetAllSessionsCached(ctx context.Context, ttl time.Duration) ([]Session, error) {
	var allSessions []Session

	// Check if all enabled servers have fresh cache
	allFresh := true
	enabledClients := m.GetEnabledClients()

	for serverID := range enabledClients {
		if !m.cache.IsFresh(serverID) {
			allFresh = false
			break
		}
	}

	// If all fresh, return from cache immediately
	if allFresh && len(enabledClients) > 0 {
		entries := m.cache.GetAll()
		for serverID := range enabledClients {
			if entry, exists := entries[serverID]; exists {
				if sessions, ok := entry.Sessions.([]Session); ok {
					allSessions = append(allSessions, sessions...)
				}
			}
		}
		return allSessions, nil
	}

	// Otherwise, trigger refresh in background if any are stale
	// For first request (cold start), wait for refresh
	if !allFresh {
		// Check if we have ANY cache entries
		hasCachedData := false
		entries := m.cache.GetAll()
		for serverID := range enabledClients {
			if _, exists := entries[serverID]; exists {
				hasCachedData = true
				break
			}
		}

		if !hasCachedData {
			// Cold start: block and wait for refresh
			err := m.UpdateAllSessions(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			// Have stale data: refresh in background, return cached
			go m.UpdateAllSessions(context.Background())
		}
	}

	// Return from cache
	entries := m.cache.GetAll()
	for serverID := range enabledClients {
		if entry, exists := entries[serverID]; exists {
			if sessions, ok := entry.Sessions.([]Session); ok {
				allSessions = append(allSessions, sessions...)
			}
		}
	}

	return allSessions, nil
}

// UpdateAllSessions force-refreshes cache from all servers in parallel
func (m *MultiServerManager) UpdateAllSessions(ctx context.Context) error {
	enabledClients := m.GetEnabledClients()

	var wg sync.WaitGroup

	// Fan out to all servers in parallel
	for serverID, client := range enabledClients {
		wg.Add(1)
		go func(sID string, c MediaServerClient) {
			defer wg.Done()

			// Fetch sessions with timeout
			sessions, err := c.GetActiveSessions()

			if err != nil {
				// Mark as degraded but keep last known sessions
				entry, exists := m.cache.Get(sID)
				if exists {
					m.cache.SetWithError(sID, entry.Sessions, entry.ServerType, sessioncache.Degraded, err)
				} else {
					m.cache.SetWithError(sID, []Session{}, string(c.GetServerType()), sessioncache.Degraded, err)
				}
				return
			}

			// Update cache with fresh data
			m.cache.Set(sID, sessions, string(c.GetServerType()), sessioncache.Fresh)
		}(serverID, client)
	}

	wg.Wait()
	return nil
}

// PublishSessionsToCache stores processed sessions in cache
func (m *MultiServerManager) PublishSessionsToCache(serverID string, sessions []Session, status sessioncache.CacheStatus) {
	if m.cache != nil {
		var serverType string
		if config, ok := m.configs[serverID]; ok {
			serverType = string(config.Type)
		}
		m.cache.Set(serverID, sessions, serverType, status)
	}
}

// GetServerConfigs returns all server configurations
func (m *MultiServerManager) GetServerConfigs() map[string]ServerConfig {
	return m.configs
}

// GetServerHealth checks health of all servers
func (m *MultiServerManager) GetServerHealth() map[string]*ServerHealth {
	health := make(map[string]*ServerHealth)

	// Iterate over configs so servers without clients are also reported
	for serverID, cfg := range m.configs {
		if client, ok := m.clients[serverID]; ok && client != nil {
			serverHealth, err := client.CheckHealth()
			if err != nil {
				health[serverID] = &ServerHealth{
					ServerID:    serverID,
					ServerType:  cfg.Type,
					ServerName:  cfg.Name,
					IsReachable: false,
					Error:       err.Error(),
				}
				continue
			}
			health[serverID] = serverHealth
			continue
		}
		// No client registered: mark as unavailable
		health[serverID] = &ServerHealth{
			ServerID:    serverID,
			ServerType:  cfg.Type,
			ServerName:  cfg.Name,
			IsReachable: false,
			Error:       "no client registered",
		}
	}
	return health
}
