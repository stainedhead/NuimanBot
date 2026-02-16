package storage

import (
	"context"
	"nuimanbot/internal/domain"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileAuditRepository_Append(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileAuditRepository(basePath)

	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "user-123",
		Action:    "login",
		Resource:  "authentication",
		Outcome:   "success",
		Details:   map[string]any{"method": "password"},
		SourceIP:  "192.168.1.1",
		Platform:  domain.PlatformCLI,
	}

	ctx := context.Background()
	err := repo.Append(ctx, event)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify can query event
	filter := domain.AuditFilter{
		UserID: "user-123",
	}
	events, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
	if events[0].Action != "login" {
		t.Errorf("expected action 'login', got %s", events[0].Action)
	}
}

func TestFileAuditRepository_Query(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileAuditRepository(basePath)

	ctx := context.Background()

	// Append multiple events
	events := []*domain.AuditEvent{
		{
			Timestamp: time.Now(),
			UserID:    "user-123",
			Action:    "login",
			Resource:  "authentication",
			Outcome:   "success",
			Details:   map[string]any{},
			SourceIP:  "192.168.1.1",
			Platform:  domain.PlatformCLI,
		},
		{
			Timestamp: time.Now(),
			UserID:    "user-123",
			Action:    "skill_execution",
			Resource:  "calculator",
			Outcome:   "success",
			Details:   map[string]any{},
			SourceIP:  "192.168.1.1",
			Platform:  domain.PlatformCLI,
		},
		{
			Timestamp: time.Now(),
			UserID:    "user-456",
			Action:    "login",
			Resource:  "authentication",
			Outcome:   "failure",
			Details:   map[string]any{},
			SourceIP:  "192.168.1.2",
			Platform:  domain.PlatformSlack,
		},
	}

	for _, event := range events {
		err := repo.Append(ctx, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query all events
	filter := domain.AuditFilter{}
	allEvents, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(allEvents) != 3 {
		t.Errorf("expected 3 events, got %d", len(allEvents))
	}

	// Query by user
	filter = domain.AuditFilter{UserID: "user-123"}
	userEvents, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query by user failed: %v", err)
	}

	if len(userEvents) != 2 {
		t.Errorf("expected 2 events for user-123, got %d", len(userEvents))
	}

	// Query by action
	filter = domain.AuditFilter{Action: "login"}
	loginEvents, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query by action failed: %v", err)
	}

	if len(loginEvents) != 2 {
		t.Errorf("expected 2 login events, got %d", len(loginEvents))
	}

	// Query by outcome
	filter = domain.AuditFilter{Outcome: "failure"}
	failureEvents, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query by outcome failed: %v", err)
	}

	if len(failureEvents) != 1 {
		t.Errorf("expected 1 failure event, got %d", len(failureEvents))
	}
}

func TestFileAuditRepository_QueryWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileAuditRepository(basePath)

	ctx := context.Background()

	// Append 10 events
	for i := 0; i < 10; i++ {
		event := &domain.AuditEvent{
			Timestamp: time.Now(),
			UserID:    "user-123",
			Action:    "test_action",
			Resource:  "test_resource",
			Outcome:   "success",
			Details:   map[string]any{"index": i},
			SourceIP:  "192.168.1.1",
			Platform:  domain.PlatformCLI,
		}
		err := repo.Append(ctx, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Query with limit
	filter := domain.AuditFilter{
		Limit: 5,
	}
	events, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}

	// Query with offset and limit
	filter = domain.AuditFilter{
		Offset: 5,
		Limit:  3,
	}
	events, err = repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query with offset failed: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestFileAuditRepository_ConcurrentAppend(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileAuditRepository(basePath)

	ctx := context.Background()

	// Append events concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	eventsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &domain.AuditEvent{
					Timestamp: time.Now(),
					UserID:    "user-concurrent",
					Action:    "concurrent_action",
					Resource:  "test",
					Outcome:   "success",
					Details:   map[string]any{"goroutine": goroutineID, "event": j},
					SourceIP:  "192.168.1.1",
					Platform:  domain.PlatformCLI,
				}
				err := repo.Append(ctx, event)
				if err != nil {
					t.Errorf("Concurrent append failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were written
	filter := domain.AuditFilter{}
	events, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	expectedCount := numGoroutines * eventsPerGoroutine
	if len(events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(events))
	}
}

func TestFileAuditRepository_ReverseChronologicalOrder(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileAuditRepository(basePath)

	ctx := context.Background()

	// Append events with distinct timestamps
	now := time.Now()
	for i := 0; i < 5; i++ {
		event := &domain.AuditEvent{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			UserID:    "user-123",
			Action:    "test",
			Resource:  "test",
			Outcome:   "success",
			Details:   map[string]any{"order": i},
			SourceIP:  "192.168.1.1",
			Platform:  domain.PlatformCLI,
		}
		err := repo.Append(ctx, event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure distinct timestamps
	}

	// Query and verify reverse chronological order
	filter := domain.AuditFilter{}
	events, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Events should be newest first
	for i := 0; i < len(events)-1; i++ {
		if events[i].Timestamp.Before(events[i+1].Timestamp) {
			t.Error("events not in reverse chronological order")
		}
	}
}
