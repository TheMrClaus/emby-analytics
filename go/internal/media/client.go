package media

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
}

// NewMultiServerManager creates a new multi-server manager
func NewMultiServerManager() *MultiServerManager {
	return &MultiServerManager{
		clients: make(map[string]MediaServerClient),
		configs: make(map[string]ServerConfig),
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

// GetEnabledClients returns only enabled clients
func (m *MultiServerManager) GetEnabledClients() map[string]MediaServerClient {
	enabled := make(map[string]MediaServerClient)
	for serverID, client := range m.clients {
		if config, exists := m.configs[serverID]; exists && config.Enabled {
			enabled[serverID] = client
		}
	}
	return enabled
}

// GetAllSessions aggregates sessions from all enabled servers
func (m *MultiServerManager) GetAllSessions() ([]Session, error) {
	var allSessions []Session
	
	for serverID, client := range m.GetEnabledClients() {
		sessions, err := client.GetActiveSessions()
		if err != nil {
			// Log error but continue with other servers
			continue
		}
		allSessions = append(allSessions, sessions...)
	}
	
	return allSessions, nil
}

// GetServerConfigs returns all server configurations
func (m *MultiServerManager) GetServerConfigs() map[string]ServerConfig {
	return m.configs
}

// GetServerHealth checks health of all servers
func (m *MultiServerManager) GetServerHealth() map[string]*ServerHealth {
	health := make(map[string]*ServerHealth)
	
	for serverID, client := range m.clients {
		serverHealth, err := client.CheckHealth()
		if err != nil {
			// Create failed health status
			config := m.configs[serverID]
			serverHealth = &ServerHealth{
				ServerID:    serverID,
				ServerType:  config.Type,
				ServerName:  config.Name,
				IsReachable: false,
				Error:       err.Error(),
			}
		}
		health[serverID] = serverHealth
	}
	
	return health
}