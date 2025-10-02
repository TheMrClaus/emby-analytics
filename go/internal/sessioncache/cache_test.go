package sessioncache

import (
	"sync"
	"testing"
	"time"

	"emby-analytics/internal/media"
)

func TestNew(t *testing.T) {
	ttl := 5 * time.Second
	cache := New(ttl)

	if cache == nil {
		t.Fatal("New() returned nil")
	}
	if cache.ttl != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, cache.ttl)
	}
	if cache.entries == nil {
		t.Error("entries map not initialized")
	}
	if cache.metrics == nil {
		t.Error("metrics not initialized")
	}
}

func TestSetAndGet(t *testing.T) {
	cache := New(5 * time.Second)
	serverID := "test-server"

	sessions := []media.Session{
		{
			ServerID:   serverID,
			ServerType: media.ServerTypeEmby,
			SessionID:  "session1",
			UserName:   "testuser",
			ItemName:   "Test Movie",
		},
	}

	// Set sessions
	cache.Set(serverID, sessions, Fresh)

	// Get sessions
	entry, exists := cache.Get(serverID)
	if !exists {
		t.Fatal("Expected entry to exist")
	}
	if entry.ServerID != serverID {
		t.Errorf("Expected ServerID %s, got %s", serverID, entry.ServerID)
	}
	if len(entry.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(entry.Sessions))
	}
	if entry.Sessions[0].SessionID != "session1" {
		t.Errorf("Expected SessionID session1, got %s", entry.Sessions[0].SessionID)
	}
	if entry.Status != Fresh {
		t.Errorf("Expected status Fresh, got %v", entry.Status)
	}
}

func TestGetNonExistent(t *testing.T) {
	cache := New(5 * time.Second)

	entry, exists := cache.Get("nonexistent")
	if exists {
		t.Error("Expected entry not to exist")
	}
	if entry != nil {
		t.Error("Expected nil entry")
	}
}

func TestIsFresh(t *testing.T) {
	cache := New(100 * time.Millisecond)
	serverID := "test-server"

	// Initially no entry
	if cache.IsFresh(serverID) {
		t.Error("Expected false for nonexistent entry")
	}

	// Add fresh entry
	cache.Set(serverID, []media.Session{}, Fresh)
	if !cache.IsFresh(serverID) {
		t.Error("Expected true for fresh entry")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)
	if cache.IsFresh(serverID) {
		t.Error("Expected false for expired entry")
	}
}

func TestGetAll(t *testing.T) {
	cache := New(5 * time.Second)

	cache.Set("server1", []media.Session{}, Fresh)
	cache.Set("server2", []media.Session{}, Stale)
	cache.Set("server3", []media.Session{}, Degraded)

	all := cache.GetAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(all))
	}

	// Verify entries exist
	for _, serverID := range []string{"server1", "server2", "server3"} {
		if _, exists := all[serverID]; !exists {
			t.Errorf("Expected %s to exist in GetAll()", serverID)
		}
	}
}

func TestMergeAdd(t *testing.T) {
	cache := New(5 * time.Second)
	serverID := "test-server"

	session1 := media.Session{
		ServerID:   serverID,
		ServerType: media.ServerTypeEmby,
		SessionID:  "session1",
		UserName:   "user1",
	}

	// Merge into empty cache
	cache.Merge(serverID, session1, MergeAdd)

	entry, exists := cache.Get(serverID)
	if !exists {
		t.Fatal("Expected entry to exist")
	}
	if len(entry.Sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(entry.Sessions))
	}
	if entry.Sessions[0].SessionID != "session1" {
		t.Errorf("Expected session1, got %s", entry.Sessions[0].SessionID)
	}

	// Add another session
	session2 := media.Session{
		ServerID:   serverID,
		ServerType: media.ServerTypeEmby,
		SessionID:  "session2",
		UserName:   "user2",
	}
	cache.Merge(serverID, session2, MergeAdd)

	entry, _ = cache.Get(serverID)
	if len(entry.Sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(entry.Sessions))
	}
}

func TestMergeUpdate(t *testing.T) {
	cache := New(5 * time.Second)
	serverID := "test-server"

	session := media.Session{
		ServerID:   serverID,
		ServerType: media.ServerTypeEmby,
		SessionID:  "session1",
		UserName:   "user1",
		ItemName:   "Movie A",
	}

	cache.Merge(serverID, session, MergeAdd)

	// Update the session
	updatedSession := media.Session{
		ServerID:   serverID,
		ServerType: media.ServerTypeEmby,
		SessionID:  "session1",
		UserName:   "user1",
		ItemName:   "Movie B", // Changed
	}
	cache.Merge(serverID, updatedSession, MergeUpdate)

	entry, _ := cache.Get(serverID)
	if len(entry.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(entry.Sessions))
	}
	if entry.Sessions[0].ItemName != "Movie B" {
		t.Errorf("Expected ItemName 'Movie B', got '%s'", entry.Sessions[0].ItemName)
	}
}

func TestMergeRemove(t *testing.T) {
	cache := New(5 * time.Second)
	serverID := "test-server"

	// Add two sessions
	session1 := media.Session{
		ServerID:   serverID,
		ServerType: media.ServerTypeEmby,
		SessionID:  "session1",
	}
	session2 := media.Session{
		ServerID:   serverID,
		ServerType: media.ServerTypeEmby,
		SessionID:  "session2",
	}

	cache.Merge(serverID, session1, MergeAdd)
	cache.Merge(serverID, session2, MergeAdd)

	entry, _ := cache.Get(serverID)
	if len(entry.Sessions) != 2 {
		t.Fatalf("Expected 2 sessions, got %d", len(entry.Sessions))
	}

	// Remove session1
	cache.Merge(serverID, session1, MergeRemove)

	entry, _ = cache.Get(serverID)
	if len(entry.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(entry.Sessions))
	}
	if entry.Sessions[0].SessionID != "session2" {
		t.Errorf("Expected session2 to remain, got %s", entry.Sessions[0].SessionID)
	}
}

func TestConcurrency(t *testing.T) {
	cache := New(5 * time.Second)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			serverID := "server-" + string(rune('0'+id))
			sessions := []media.Session{
				{ServerID: serverID, SessionID: "session1"},
			}
			cache.Set(serverID, sessions, Fresh, media.ServerTypeEmby)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			serverID := "server-" + string(rune('0'+id))
			cache.Get(serverID)
		}(i)
	}

	wg.Wait()
}

func TestMetrics(t *testing.T) {
	cache := New(5 * time.Second)

	// Initial metrics should be zero
	metrics := cache.GetMetrics()
	if metrics.Hits != 0 || metrics.Misses != 0 {
		t.Error("Expected zero metrics initially")
	}

	// Set and get should increment metrics
	cache.Set("server1", []media.Session{}, Fresh)
	cache.Get("server1") // Hit
	cache.Get("server2") // Miss

	metrics = cache.GetMetrics()
	if metrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", metrics.Misses)
	}
	if metrics.Refreshes != 1 {
		t.Errorf("Expected 1 refresh, got %d", metrics.Refreshes)
	}
}

func TestClear(t *testing.T) {
	cache := New(5 * time.Second)

	cache.Set("server1", []media.Session{}, Fresh)
	cache.Set("server2", []media.Session{}, Fresh)

	all := cache.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 entries before clear, got %d", len(all))
	}

	cache.Clear()

	all = cache.GetAll()
	if len(all) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(all))
	}
}

func TestSetWithError(t *testing.T) {
	cache := New(5 * time.Second)
	serverID := "test-server"

	// Set with error (degraded mode)
	cache.SetWithError(serverID, []media.Session{}, Degraded, nil)

	entry, exists := cache.Get(serverID)
	if !exists {
		t.Fatal("Expected entry to exist")
	}
	if entry.Status != Degraded {
		t.Errorf("Expected status Degraded, got %v", entry.Status)
	}

	// Check metrics
	metrics := cache.GetMetrics()
	if metrics.Refreshes != 1 {
		t.Errorf("Expected 1 refresh, got %d", metrics.Refreshes)
	}
}

func TestCacheStatusString(t *testing.T) {
	tests := []struct {
		status   CacheStatus
		expected string
	}{
		{Fresh, "fresh"},
		{Stale, "stale"},
		{Degraded, "degraded"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, got)
		}
	}
}
