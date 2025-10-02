package sessioncache

import (
	"sync"
	"time"
)

// CacheStatus represents the health/freshness status of a cache entry
type CacheStatus int

const (
	Fresh    CacheStatus = iota // Within TTL, no errors
	Stale                        // Past TTL, needs refresh
	Degraded                     // Server unreachable, serving last known state
)

// String returns human-readable status
func (s CacheStatus) String() string {
	switch s {
	case Fresh:
		return "fresh"
	case Stale:
		return "stale"
	case Degraded:
		return "degraded"
	default:
		return "unknown"
	}
}

// CacheEntry represents a single cache entry for a server's sessions
// The Sessions field is interface{} to avoid circular imports
type CacheEntry struct {
	Sessions   interface{} `json:"sessions"` // []media.Session but stored as interface{}
	Timestamp  time.Time   `json:"timestamp"`
	Status     CacheStatus `json:"status"`
	ServerID   string      `json:"server_id"`
	ServerType string      `json:"server_type"` // "emby", "plex", "jellyfin"
	Error      error       `json:"error,omitempty"`
}

// SessionCache provides thread-safe caching of session data with TTL
type SessionCache struct {
	entries map[string]*CacheEntry // Key: serverID
	mu      sync.RWMutex
	ttl     time.Duration
	metrics *internalMetrics
}

// New creates a new SessionCache with the given TTL
func New(ttl time.Duration) *SessionCache {
	return &SessionCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
		metrics: &internalMetrics{},
	}
}

// Get retrieves a cache entry for the given server
// Returns the entry and true if found, nil and false otherwise
func (c *SessionCache) Get(serverID string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[serverID]
	if exists {
		c.metrics.Hits.Add(1)
	} else {
		c.metrics.Misses.Add(1)
	}

	return entry, exists
}

// Set stores sessions for a given server with the specified status
// sessions should be []media.Session
func (c *SessionCache) Set(serverID string, sessions interface{}, status CacheStatus, serverType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[serverID] = &CacheEntry{
		Sessions:   sessions,
		Timestamp:  time.Now(),
		Status:     status,
		ServerID:   serverID,
		ServerType: serverType,
	}

	c.metrics.Refreshes.Add(1)
}

// SetWithError stores sessions with an error state (for degraded mode)
func (c *SessionCache) SetWithError(serverID string, sessions interface{}, serverType string, status CacheStatus, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[serverID] = &CacheEntry{
		Sessions:   sessions,
		Timestamp:  time.Now(),
		Status:     status,
		ServerID:   serverID,
		ServerType: serverType,
		Error:      err,
	}

	if err != nil {
		c.metrics.RefreshFailures.Add(1)
	} else {
		c.metrics.Refreshes.Add(1)
	}
}

// IsFresh checks if a cache entry is within TTL
func (c *SessionCache) IsFresh(serverID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[serverID]
	if !exists {
		return false
	}

	return entry.Status == Fresh && time.Since(entry.Timestamp) < c.ttl
}

// GetAll returns a copy of all cache entries
func (c *SessionCache) GetAll() map[string]*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create a copy to avoid concurrent modification issues
	result := make(map[string]*CacheEntry, len(c.entries))
	for k, v := range c.entries {
		// Create a shallow copy of the entry
		entryCopy := *v
		result[k] = &entryCopy
	}

	return result
}

// GetMetrics returns a snapshot of cache metrics
func (c *SessionCache) GetMetrics() CacheMetrics {
	return CacheMetrics{
		Hits:             c.metrics.Hits.Load(),
		Misses:           c.metrics.Misses.Load(),
		Refreshes:        c.metrics.Refreshes.Load(),
		RefreshFailures:  c.metrics.RefreshFailures.Load(),
		WebSocketUpdates: c.metrics.WebSocketUpdates.Load(),
	}
}

// Clear removes all cache entries (useful for testing)
func (c *SessionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}
