package audit_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/audit"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	return db
}

func TestNewSQLiteAuditor(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() returned error: %v", err)
	}

	if auditor == nil {
		t.Fatal("NewSQLiteAuditor() returned nil auditor")
	}
}

func TestSQLiteAuditor_Audit_BasicEvent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()
	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "user123",
		Action:    "login",
		Resource:  "system",
		Outcome:   "success",
	}

	err = auditor.Audit(ctx, event)
	if err != nil {
		t.Fatalf("Audit() returned error: %v", err)
	}

	// Verify event was inserted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query audit_log: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event in audit_log, got %d", count)
	}
}

func TestSQLiteAuditor_Audit_WithDetails(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()
	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "user456",
		Action:    "file_access",
		Resource:  "/sensitive/data.txt",
		Outcome:   "denied",
		Details: map[string]interface{}{
			"ip_address": "192.168.1.1",
			"reason":     "insufficient_permissions",
		},
	}

	err = auditor.Audit(ctx, event)
	if err != nil {
		t.Fatalf("Audit() returned error: %v", err)
	}

	// Verify details were stored
	var details string
	err = db.QueryRow("SELECT details FROM audit_log WHERE user_id = ?", "user456").Scan(&details)
	if err != nil {
		t.Fatalf("Failed to query details: %v", err)
	}

	if details == "" || details == "{}" {
		t.Error("Expected non-empty details")
	}
}

func TestSQLiteAuditor_Audit_NilEvent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()
	err = auditor.Audit(ctx, nil)
	if err == nil {
		t.Error("Audit() should error for nil event")
	}
}

func TestSQLiteAuditor_Query_ByUserID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	// Insert test events
	events := []*domain.AuditEvent{
		{Timestamp: time.Now(), UserID: "user1", Action: "login", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user2", Action: "login", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user1", Action: "logout", Outcome: "success"},
	}

	for _, event := range events {
		auditor.Audit(ctx, event)
	}

	// Query events for user1
	criteria := audit.AuditQueryCriteria{
		UserID: "user1",
	}

	results, err := auditor.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Query() returned error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 events for user1, got %d", len(results))
	}
}

func TestSQLiteAuditor_Query_ByAction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	// Insert test events
	events := []*domain.AuditEvent{
		{Timestamp: time.Now(), UserID: "user1", Action: "login", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user2", Action: "file_access", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user3", Action: "login", Outcome: "failure"},
	}

	for _, event := range events {
		auditor.Audit(ctx, event)
	}

	// Query login events
	criteria := audit.AuditQueryCriteria{
		Action: "login",
	}

	results, err := auditor.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Query() returned error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 login events, got %d", len(results))
	}
}

func TestSQLiteAuditor_Query_ByOutcome(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	// Insert test events
	events := []*domain.AuditEvent{
		{Timestamp: time.Now(), UserID: "user1", Action: "login", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user2", Action: "login", Outcome: "failure"},
		{Timestamp: time.Now(), UserID: "user3", Action: "login", Outcome: "failure"},
	}

	for _, event := range events {
		auditor.Audit(ctx, event)
	}

	// Query failed logins
	criteria := audit.AuditQueryCriteria{
		Outcome: "failure",
	}

	results, err := auditor.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Query() returned error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 failed events, got %d", len(results))
	}
}

func TestSQLiteAuditor_Query_ByTimeRange(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	// Insert test events with different timestamps
	events := []*domain.AuditEvent{
		{Timestamp: yesterday, UserID: "user1", Action: "old_action", Outcome: "success"},
		{Timestamp: now, UserID: "user2", Action: "current_action", Outcome: "success"},
		{Timestamp: now, UserID: "user3", Action: "current_action2", Outcome: "success"},
	}

	for _, event := range events {
		auditor.Audit(ctx, event)
	}

	// Query events from today
	criteria := audit.AuditQueryCriteria{
		StartTime: now.Add(-1 * time.Hour),
		EndTime:   tomorrow,
	}

	results, err := auditor.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Query() returned error: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 current events, got %d", len(results))
	}
}

func TestSQLiteAuditor_Query_WithLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	// Insert 10 events
	for i := 0; i < 10; i++ {
		event := &domain.AuditEvent{
			Timestamp: time.Now(),
			UserID:    "user1",
			Action:    "action",
			Outcome:   "success",
		}
		auditor.Audit(ctx, event)
	}

	// Query with limit of 5
	criteria := audit.AuditQueryCriteria{
		UserID: "user1",
		Limit:  5,
	}

	results, err := auditor.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Query() returned error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Expected 5 events (limited), got %d", len(results))
	}
}

func TestSQLiteAuditor_DeleteOldEvents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	// Insert events with different timestamps
	oldTime := time.Now().AddDate(0, 0, -100)   // 100 days ago
	recentTime := time.Now().AddDate(0, 0, -10) // 10 days ago

	events := []*domain.AuditEvent{
		{Timestamp: oldTime, UserID: "user1", Action: "old_event", Outcome: "success"},
		{Timestamp: oldTime, UserID: "user2", Action: "old_event2", Outcome: "success"},
		{Timestamp: recentTime, UserID: "user3", Action: "recent_event", Outcome: "success"},
	}

	for _, event := range events {
		auditor.Audit(ctx, event)
	}

	// Delete events older than 90 days
	rowsDeleted, err := auditor.DeleteOldEvents(ctx, 90)
	if err != nil {
		t.Fatalf("DeleteOldEvents() returned error: %v", err)
	}

	if rowsDeleted != 2 {
		t.Errorf("Expected 2 rows deleted, got %d", rowsDeleted)
	}

	// Verify only recent event remains
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query audit_log: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event remaining, got %d", count)
	}
}

func TestSQLiteAuditor_Query_MultipleFilters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	ctx := context.Background()

	// Insert test events
	events := []*domain.AuditEvent{
		{Timestamp: time.Now(), UserID: "user1", Action: "login", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user1", Action: "login", Outcome: "failure"},
		{Timestamp: time.Now(), UserID: "user1", Action: "logout", Outcome: "success"},
		{Timestamp: time.Now(), UserID: "user2", Action: "login", Outcome: "success"},
	}

	for _, event := range events {
		auditor.Audit(ctx, event)
	}

	// Query user1's successful login
	criteria := audit.AuditQueryCriteria{
		UserID:  "user1",
		Action:  "login",
		Outcome: "success",
	}

	results, err := auditor.Query(ctx, criteria)
	if err != nil {
		t.Fatalf("Query() returned error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 event, got %d", len(results))
	}

	if results[0].UserID != "user1" || results[0].Action != "login" || results[0].Outcome != "success" {
		t.Error("Query returned wrong event")
	}
}

func TestSQLiteAuditor_SchemaInitialization(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create auditor (should initialize schema)
	_, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		t.Fatalf("NewSQLiteAuditor() failed: %v", err)
	}

	// Verify table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='audit_log'").Scan(&tableName)
	if err != nil {
		t.Fatalf("audit_log table not created: %v", err)
	}

	if tableName != "audit_log" {
		t.Errorf("Expected table name 'audit_log', got '%s'", tableName)
	}

	// Verify indexes exist
	indexes := []string{
		"idx_audit_timestamp",
		"idx_audit_user",
		"idx_audit_action",
		"idx_audit_outcome",
	}

	for _, indexName := range indexes {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&name)
		if err != nil {
			t.Errorf("Index %s not created: %v", indexName, err)
		}
	}
}
