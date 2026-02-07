package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"nuimanbot/internal/domain"
)

// SQLiteAuditor implements audit logging with SQLite persistence.
type SQLiteAuditor struct {
	db *sql.DB
}

// NewSQLiteAuditor creates a new SQLite-based auditor.
func NewSQLiteAuditor(db *sql.DB) (*SQLiteAuditor, error) {
	auditor := &SQLiteAuditor{db: db}

	// Create audit table if it doesn't exist
	if err := auditor.initializeSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize audit schema: %w", err)
	}

	return auditor, nil
}

// initializeSchema creates the audit_log table and indexes.
func (a *SQLiteAuditor) initializeSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		user_id TEXT,
		action TEXT NOT NULL,
		resource TEXT,
		outcome TEXT NOT NULL,
		details TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action, timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_outcome ON audit_log(outcome, timestamp);
	`

	_, err := a.db.Exec(schema)
	return err
}

// Audit logs a security event to the database.
func (a *SQLiteAuditor) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if event == nil {
		return fmt.Errorf("audit event cannot be nil")
	}

	// Marshal details to JSON
	var detailsJSON string
	if event.Details != nil {
		detailsBytes, err := json.Marshal(event.Details)
		if err != nil {
			slog.Warn("failed to marshal audit event details",
				"error", err,
				"action", event.Action,
			)
			detailsJSON = "{}"
		} else {
			detailsJSON = string(detailsBytes)
		}
	}

	// Insert audit event
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO audit_log (timestamp, user_id, action, resource, outcome, details)
		VALUES (?, ?, ?, ?, ?, ?)
	`, event.Timestamp, event.UserID, event.Action, event.Resource, event.Outcome, detailsJSON)

	if err != nil {
		slog.Error("failed to insert audit event",
			"error", err,
			"action", event.Action,
			"user_id", event.UserID,
		)
		return fmt.Errorf("failed to insert audit event: %w", err)
	}

	slog.Debug("audit event logged",
		"action", event.Action,
		"user_id", event.UserID,
		"outcome", event.Outcome,
	)

	return nil
}

// Query retrieves audit events matching the given criteria.
func (a *SQLiteAuditor) Query(ctx context.Context, criteria *AuditQueryCriteria) ([]domain.AuditEvent, error) {
	query := `
		SELECT id, timestamp, user_id, action, resource, outcome, details
		FROM audit_log
		WHERE 1=1
	`
	args := []interface{}{}

	// Build WHERE clause dynamically
	if criteria.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, criteria.UserID)
	}

	if criteria.Action != "" {
		query += " AND action = ?"
		args = append(args, criteria.Action)
	}

	if criteria.Outcome != "" {
		query += " AND outcome = ?"
		args = append(args, criteria.Outcome)
	}

	if !criteria.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, criteria.StartTime)
	}

	if !criteria.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, criteria.EndTime)
	}

	// Order by timestamp descending (newest first)
	query += " ORDER BY timestamp DESC"

	// Apply limit
	if criteria.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, criteria.Limit)
	}

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []domain.AuditEvent
	for rows.Next() {
		var event domain.AuditEvent
		var id int64
		var detailsJSON sql.NullString

		err := rows.Scan(
			&id,
			&event.Timestamp,
			&event.UserID,
			&event.Action,
			&event.Resource,
			&event.Outcome,
			&detailsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}

		// Unmarshal details if present
		if detailsJSON.Valid && detailsJSON.String != "" {
			var details map[string]interface{}
			if err := json.Unmarshal([]byte(detailsJSON.String), &details); err != nil {
				slog.Warn("failed to unmarshal audit event details",
					"error", err,
					"event_id", id,
				)
			} else {
				event.Details = details
			}
		}

		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit events: %w", err)
	}

	return events, nil
}

// DeleteOldEvents removes audit events older than the retention period.
func (a *SQLiteAuditor) DeleteOldEvents(ctx context.Context, retentionDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	result, err := a.db.ExecContext(ctx, `
		DELETE FROM audit_log
		WHERE timestamp < ?
	`, cutoffTime)

	if err != nil {
		return 0, fmt.Errorf("failed to delete old audit events: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	slog.Info("deleted old audit events",
		"cutoff_date", cutoffTime.Format("2006-01-02"),
		"rows_deleted", rowsAffected,
	)

	return rowsAffected, nil
}

// AuditQueryCriteria defines criteria for querying audit events.
type AuditQueryCriteria struct {
	UserID    string
	Action    string
	Outcome   string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}
