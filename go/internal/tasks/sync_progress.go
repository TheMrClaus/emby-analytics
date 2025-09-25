package tasks

import (
	"sort"
	"sync"
	"time"
)

// ServerSyncProgress tracks per-server sync status for multi-server ingestion.
type ServerSyncProgress struct {
	ServerID   string    `json:"server_id"`
	ServerName string    `json:"server_name,omitempty"`
	Total      int       `json:"total"`
	Processed  int       `json:"processed"`
	Stage      string    `json:"stage,omitempty"`
	Running    bool      `json:"running"`
	Done       bool      `json:"done"`
	Error      string    `json:"error,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

var (
	syncProgressMu sync.RWMutex
	syncProgress   = make(map[string]ServerSyncProgress)
)

// StartServerSyncProgress initializes progress tracking for a server.
func StartServerSyncProgress(serverID, serverName string) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	syncProgress[serverID] = ServerSyncProgress{
		ServerID:   serverID,
		ServerName: serverName,
		Stage:      "Fetching library metadata...",
		Running:    true,
		UpdatedAt:  time.Now(),
	}
}

// UpdateServerSyncTotals sets the expected item count for a server.
func UpdateServerSyncTotals(serverID string, total int) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		return
	}
	if total >= 0 {
		p.Total = total
	}
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// SetServerSyncProcessed sets the processed count explicitly.
func SetServerSyncProcessed(serverID string, processed int) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		return
	}
	if processed < 0 {
		processed = 0
	}
	p.Processed = processed
	if p.Processed > p.Total && p.Total > 0 {
		p.Processed = p.Total
	}
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// IncrementServerSyncProcessed increments processed counter.
func IncrementServerSyncProcessed(serverID string, delta int) {
	if delta == 0 {
		return
	}
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		return
	}
	p.Processed += delta
	if p.Processed < 0 {
		p.Processed = 0
	}
	if p.Total > 0 && p.Processed > p.Total {
		p.Processed = p.Total
	}
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// SetServerSyncStage updates the descriptive stage text.
func SetServerSyncStage(serverID, stage string) {
	if stage == "" {
		return
	}
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		return
	}
	p.Stage = stage
	if !p.Done {
		p.Running = true
	}
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// CompleteServerSyncProgress marks the sync as completed successfully.
func CompleteServerSyncProgress(serverID string) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		return
	}
	if p.Total > 0 && p.Processed < p.Total {
		p.Processed = p.Total
	}
	p.Running = false
	p.Done = true
	p.Stage = "Full sync complete"
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// FailServerSyncProgress marks the sync as failed with an error.
func FailServerSyncProgress(serverID string, err error) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		p = ServerSyncProgress{ServerID: serverID}
	}
	p.Running = false
	p.Done = true
	if err != nil {
		p.Error = err.Error()
	}
	if p.Stage == "" {
		p.Stage = "Sync failed"
	}
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// ResetServerSyncProgress removes tracking for a server.
func ResetServerSyncProgress(serverID string) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	delete(syncProgress, serverID)
}

// CancelServerSyncProgress marks the sync as cancelled without treating it as an error.
func CancelServerSyncProgress(serverID string, reason string) {
	syncProgressMu.Lock()
	defer syncProgressMu.Unlock()
	p, ok := syncProgress[serverID]
	if !ok {
		p = ServerSyncProgress{ServerID: serverID}
	}
	p.Running = false
	p.Done = true
	p.Error = ""
	if reason != "" {
		p.Stage = reason
	}
	p.UpdatedAt = time.Now()
	syncProgress[serverID] = p
}

// GetServerSyncProgressSnapshot returns a copy of current progress entries.
func GetServerSyncProgressSnapshot() []ServerSyncProgress {
	syncProgressMu.RLock()
	defer syncProgressMu.RUnlock()
	out := make([]ServerSyncProgress, 0, len(syncProgress))
	for _, p := range syncProgress {
		cp := p
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServerName == out[j].ServerName {
			return out[i].ServerID < out[j].ServerID
		}
		return out[i].ServerName < out[j].ServerName
	})
	return out
}
