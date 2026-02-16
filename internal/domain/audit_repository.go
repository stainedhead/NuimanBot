package domain

import "context"

// AuditFilter defines criteria for querying audit events
type AuditFilter struct {
	UserID  string // Filter by user ID (empty = all users)
	Action  string // Filter by action (empty = all actions)
	Outcome string // Filter by outcome (empty = all outcomes)
	Limit   int    // Maximum number of results (0 = no limit)
	Offset  int    // Number of results to skip
}

// AuditRepository defines operations for audit log persistence.
type AuditRepository interface {
	// Append adds a new audit entry (append-only, concurrent-safe).
	Append(ctx context.Context, entry *AuditEvent) error

	// Query retrieves audit entries matching filter.
	// Results are returned in reverse chronological order (newest first).
	Query(ctx context.Context, filter AuditFilter) ([]*AuditEvent, error)
}
