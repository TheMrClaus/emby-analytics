package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CleanupJob represents an audit log entry for cleanup operations
type CleanupJob struct {
	ID               string    `json:"id"`
	OperationType    string    `json:"operation_type"`
	Status           string    `json:"status"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	TotalItemsChecked int      `json:"total_items_checked"`
	ItemsProcessed   int       `json:"items_processed"`
	Summary          string    `json:"summary,omitempty"`
	CreatedBy        string    `json:"created_by,omitempty"`
}

// CleanupAuditItem represents an individual item action in a cleanup job
type CleanupAuditItem struct {
	ID           int64     `json:"id"`
	JobID        string    `json:"job_id"`
	ActionType   string    `json:"action_type"`
	ItemID       string    `json:"item_id"`
	ItemName     string    `json:"item_name,omitempty"`
	ItemType     string    `json:"item_type,omitempty"`
	TargetItemID string    `json:"target_item_id,omitempty"`
	Metadata     string    `json:"metadata,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// CleanupLogger handles audit logging for cleanup operations
type CleanupLogger struct {
	db    *sql.DB
	jobID string
}

// NewCleanupLogger creates a new cleanup audit logger
func NewCleanupLogger(db *sql.DB, operationType string, createdBy string) (*CleanupLogger, error) {
	jobID := uuid.New().String()
	
	_, err := db.Exec(`
		INSERT INTO cleanup_jobs (id, operation_type, status, started_at, created_by)
		VALUES (?, ?, 'running', ?, ?)
	`, jobID, operationType, time.Now().Unix(), createdBy)
	
	if err != nil {
		return nil, err
	}
	
	return &CleanupLogger{db: db, jobID: jobID}, nil
}

// LogItemAction logs an individual item action
func (cl *CleanupLogger) LogItemAction(actionType, itemID, itemName, itemType, targetItemID string, metadata map[string]interface{}) error {
	var metadataJSON string
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal audit metadata: %w", err)
		}
		metadataJSON = string(b)
	}
	
	_, err := cl.db.Exec(`
		INSERT INTO cleanup_audit_items (job_id, action_type, item_id, item_name, item_type, target_item_id, metadata, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, cl.jobID, actionType, itemID, itemName, itemType, targetItemID, metadataJSON, time.Now().Unix())
	
	return err
}

// CompleteJob marks the job as completed with summary stats
func (cl *CleanupLogger) CompleteJob(totalChecked, itemsProcessed int, summary map[string]interface{}) error {
	var summaryJSON string
	if summary != nil {
		if b, err := json.Marshal(summary); err == nil {
			summaryJSON = string(b)
		}
	}
	
	_, err := cl.db.Exec(`
		UPDATE cleanup_jobs 
		SET status = 'completed', completed_at = ?, total_items_checked = ?, items_processed = ?, summary = ?
		WHERE id = ?
	`, time.Now().Unix(), totalChecked, itemsProcessed, summaryJSON, cl.jobID)
	
	return err
}

// FailJob marks the job as failed
func (cl *CleanupLogger) FailJob(errorMsg string) error {
	summary := map[string]interface{}{"error": errorMsg}
	summaryJSON, _ := json.Marshal(summary)
	
	_, err := cl.db.Exec(`
		UPDATE cleanup_jobs 
		SET status = 'failed', completed_at = ?, summary = ?
		WHERE id = ?
	`, time.Now().Unix(), string(summaryJSON), cl.jobID)
	
	return err
}

// GetJobID returns the job ID for reference
func (cl *CleanupLogger) GetJobID() string {
	return cl.jobID
}

// GetCleanupJobs retrieves recent cleanup jobs
func GetCleanupJobs(db *sql.DB, limit int) ([]CleanupJob, error) {
	if limit <= 0 {
		limit = 50
	}
	
	rows, err := db.Query(`
		SELECT id, operation_type, status, started_at, completed_at, 
		       total_items_checked, items_processed, summary, created_by
		FROM cleanup_jobs 
		ORDER BY started_at DESC 
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var jobs []CleanupJob
	for rows.Next() {
		var job CleanupJob
		var startedAtUnix int64
		var completedAtUnix *int64
		
		err := rows.Scan(&job.ID, &job.OperationType, &job.Status, 
			&startedAtUnix, &completedAtUnix, &job.TotalItemsChecked, &job.ItemsProcessed, 
			&job.Summary, &job.CreatedBy)
		if err != nil {
			continue
		}
		
		job.StartedAt = time.Unix(startedAtUnix, 0)
		if completedAtUnix != nil {
			t := time.Unix(*completedAtUnix, 0)
			job.CompletedAt = &t
		}
		
		jobs = append(jobs, job)
	}
	
	return jobs, nil
}

// GetCleanupJobDetails retrieves details for a specific job including all item actions
func GetCleanupJobDetails(db *sql.DB, jobID string) (*CleanupJob, []CleanupAuditItem, error) {
	// Get job info
	var job CleanupJob
	var startedAtUnix, completedAtUnix *int64
	
	err := db.QueryRow(`
		SELECT id, operation_type, status, started_at, completed_at,
		       total_items_checked, items_processed, summary, created_by
		FROM cleanup_jobs WHERE id = ?
	`, jobID).Scan(&job.ID, &job.OperationType, &job.Status,
		&startedAtUnix, &completedAtUnix, &job.TotalItemsChecked, 
		&job.ItemsProcessed, &job.Summary, &job.CreatedBy)
	
	if err != nil {
		return nil, nil, err
	}
	
	if startedAtUnix != nil {
		job.StartedAt = time.Unix(*startedAtUnix, 0)
	}
	if completedAtUnix != nil {
		t := time.Unix(*completedAtUnix, 0)
		job.CompletedAt = &t
	}
	
	// Get audit items
	rows, err := db.Query(`
		SELECT id, job_id, action_type, item_id, item_name, item_type, 
		       target_item_id, metadata, timestamp
		FROM cleanup_audit_items 
		WHERE job_id = ? 
		ORDER BY timestamp DESC
	`, jobID)
	if err != nil {
		return &job, nil, err
	}
	defer rows.Close()
	
	var items []CleanupAuditItem
	for rows.Next() {
		var item CleanupAuditItem
		var timestampUnix int64
		
		err := rows.Scan(&item.ID, &item.JobID, &item.ActionType, &item.ItemID,
			&item.ItemName, &item.ItemType, &item.TargetItemID, &item.Metadata, &timestampUnix)
		if err != nil {
			continue
		}
		
		item.Timestamp = time.Unix(timestampUnix, 0)
		items = append(items, item)
	}
	
	return &job, items, nil
}