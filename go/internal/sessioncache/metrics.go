package sessioncache

import "sync/atomic"

// CacheMetrics tracks cache performance metrics using atomic operations
type CacheMetrics struct {
	Hits             int64 `json:"hits"`
	Misses           int64 `json:"misses"`
	Refreshes        int64 `json:"refreshes"`
	RefreshFailures  int64 `json:"refresh_failures"`
	WebSocketUpdates int64 `json:"websocket_updates"`
}

// internalMetrics holds the actual atomic counters
type internalMetrics struct {
	Hits             atomic.Int64
	Misses           atomic.Int64
	Refreshes        atomic.Int64
	RefreshFailures  atomic.Int64
	WebSocketUpdates atomic.Int64
}

// HitRate returns the cache hit rate as a percentage (0-100)
func (m *CacheMetrics) HitRate() float64 {
	total := m.Hits + m.Misses

	if total == 0 {
		return 0
	}

	return float64(m.Hits) / float64(total) * 100.0
}
