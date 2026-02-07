package subagent

import (
	"context"
	"errors"
	"nuimanbot/internal/domain"
	"testing"
	"time"
)

// Mock executor for testing
type mockExecutor struct {
	executeDelay time.Duration
	executeErr   error
	result       *domain.SubagentResult
}

func (m *mockExecutor) Execute(ctx context.Context, subagentCtx domain.SubagentContext) (*domain.SubagentResult, error) {
	if m.executeDelay > 0 {
		select {
		case <-time.After(m.executeDelay):
		case <-ctx.Done():
			return &domain.SubagentResult{
				SubagentID:   subagentCtx.ID,
				Status:       domain.SubagentStatusCancelled,
				ErrorMessage: "cancelled",
			}, nil
		}
	}

	if m.executeErr != nil {
		return nil, m.executeErr
	}

	if m.result != nil {
		return m.result, nil
	}

	return &domain.SubagentResult{
		SubagentID: subagentCtx.ID,
		Status:     domain.SubagentStatusComplete,
		Output:     "completed",
	}, nil
}

func (m *mockExecutor) Cancel(ctx context.Context, subagentID string) error {
	return errors.New("not used in lifecycle manager")
}

func (m *mockExecutor) GetStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error) {
	return nil, errors.New("not used in lifecycle manager")
}

// TestLifecycleManager_Start tests starting a subagent
func TestLifecycleManager_Start(t *testing.T) {
	executor := &mockExecutor{
		result: &domain.SubagentResult{
			SubagentID: "test-1",
			Status:     domain.SubagentStatusComplete,
			Output:     "success",
		},
	}

	manager := NewLifecycleManager(executor)

	subagentCtx := domain.SubagentContext{
		ID:              "test-1",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	err := manager.Start(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify subagent is tracked
	status, err := manager.GetStatus(ctx, "test-1")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}

	if status.SubagentID != "test-1" {
		t.Errorf("SubagentID = %v, want test-1", status.SubagentID)
	}

	// Status should be running or complete
	if status.Status != domain.SubagentStatusRunning && status.Status != domain.SubagentStatusComplete {
		t.Errorf("Status = %v, want running or complete", status.Status)
	}
}

// TestLifecycleManager_Start_Background tests background execution
func TestLifecycleManager_Start_Background(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 100 * time.Millisecond,
		result: &domain.SubagentResult{
			SubagentID: "test-bg",
			Status:     domain.SubagentStatusComplete,
			Output:     "success",
		},
	}

	manager := NewLifecycleManager(executor)

	subagentCtx := domain.SubagentContext{
		ID:              "test-bg",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	err := manager.Start(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Should be running immediately (background)
	status, err := manager.GetStatus(ctx, "test-bg")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Status != domain.SubagentStatusRunning && status.Status != domain.SubagentStatusComplete {
		t.Errorf("Initial status = %v, want running or complete", status.Status)
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Should be complete now
	status, err = manager.GetStatus(ctx, "test-bg")
	if err != nil {
		t.Fatalf("GetStatus() after completion error = %v", err)
	}

	if status.Status != domain.SubagentStatusComplete {
		t.Errorf("Final status = %v, want complete", status.Status)
	}
}

// TestLifecycleManager_Cancel tests cancelling a running subagent
func TestLifecycleManager_Cancel(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 5 * time.Second, // Long delay to allow cancellation
	}

	manager := NewLifecycleManager(executor)

	subagentCtx := domain.SubagentContext{
		ID:              "test-cancel",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	err := manager.Start(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the subagent
	err = manager.Cancel(ctx, "test-cancel")
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	// Wait a bit for cancellation to propagate
	time.Sleep(100 * time.Millisecond)

	// Check status - should be cancelled
	status, err := manager.GetStatus(ctx, "test-cancel")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Status != domain.SubagentStatusCancelled {
		t.Errorf("Status after cancel = %v, want cancelled", status.Status)
	}
}

// TestLifecycleManager_GetStatus_NotFound tests querying unknown subagent
func TestLifecycleManager_GetStatus_NotFound(t *testing.T) {
	executor := &mockExecutor{}
	manager := NewLifecycleManager(executor)

	ctx := context.Background()
	_, err := manager.GetStatus(ctx, "nonexistent")

	if err == nil {
		t.Error("GetStatus() for unknown subagent should return error")
	}
}

// TestLifecycleManager_Cancel_NotFound tests cancelling unknown subagent
func TestLifecycleManager_Cancel_NotFound(t *testing.T) {
	executor := &mockExecutor{}
	manager := NewLifecycleManager(executor)

	ctx := context.Background()
	err := manager.Cancel(ctx, "nonexistent")

	if err == nil {
		t.Error("Cancel() for unknown subagent should return error")
	}
}

// TestLifecycleManager_Timeout tests timeout enforcement
func TestLifecycleManager_Timeout(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 10 * time.Second, // Very long delay
	}

	manager := NewLifecycleManager(executor)

	subagentCtx := domain.SubagentContext{
		ID:              "test-timeout",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits: domain.ResourceLimits{
			MaxTokens:    100000,
			MaxToolCalls: 50,
			Timeout:      200 * time.Millisecond, // Short timeout
		},
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	err := manager.Start(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for timeout to trigger
	time.Sleep(500 * time.Millisecond)

	// Should be cancelled/timeout
	status, err := manager.GetStatus(ctx, "test-timeout")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Status != domain.SubagentStatusCancelled && status.Status != domain.SubagentStatusTimeout {
		t.Errorf("Status after timeout = %v, want cancelled or timeout", status.Status)
	}
}

// TestLifecycleManager_MultipleSubagents tests managing multiple concurrent subagents
func TestLifecycleManager_MultipleSubagents(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 100 * time.Millisecond,
	}

	manager := NewLifecycleManager(executor)

	ctx := context.Background()

	// Start multiple subagents
	for i := 0; i < 5; i++ {
		subagentCtx := domain.SubagentContext{
			ID:              string(rune('a' + i)), // a, b, c, d, e
			ParentContextID: "parent-1",
			SkillName:       "test-skill",
			AllowedTools:    []string{},
			ResourceLimits:  domain.DefaultResourceLimits(),
			ConversationHistory: []domain.Message{
				{Role: "user", Content: "test"},
			},
			CreatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		err := manager.Start(ctx, subagentCtx)
		if err != nil {
			t.Fatalf("Start() subagent %d error = %v", i, err)
		}
	}

	// All should be running or complete
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		status, err := manager.GetStatus(ctx, id)
		if err != nil {
			t.Fatalf("GetStatus() for %s error = %v", id, err)
		}

		if status == nil {
			t.Fatalf("GetStatus() for %s returned nil", id)
		}
	}

	// Wait for all to complete
	time.Sleep(300 * time.Millisecond)

	// All should be complete
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		status, err := manager.GetStatus(ctx, id)
		if err != nil {
			t.Fatalf("GetStatus() for %s after completion error = %v", id, err)
		}

		if status.Status != domain.SubagentStatusComplete {
			t.Errorf("Final status for %s = %v, want complete", id, status.Status)
		}
	}
}

// TestLifecycleManager_ListRunning tests listing active subagents
func TestLifecycleManager_ListRunning(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 200 * time.Millisecond,
	}

	manager := NewLifecycleManager(executor)

	ctx := context.Background()

	// Start 3 subagents
	for i := 0; i < 3; i++ {
		subagentCtx := domain.SubagentContext{
			ID:              string(rune('x' + i)),
			ParentContextID: "parent-1",
			SkillName:       "test-skill",
			AllowedTools:    []string{},
			ResourceLimits:  domain.DefaultResourceLimits(),
			ConversationHistory: []domain.Message{
				{Role: "user", Content: "test"},
			},
			CreatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		err := manager.Start(ctx, subagentCtx)
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	}

	// List running
	running := manager.ListRunning(ctx)

	// Should have at least some running (might complete very fast)
	if len(running) == 0 {
		// Give them a moment
		time.Sleep(50 * time.Millisecond)
		running = manager.ListRunning(ctx)
	}

	// Should have between 0 and 3 running
	if len(running) > 3 {
		t.Errorf("ListRunning() returned %d, want <= 3", len(running))
	}
}

// TestLifecycleManager_MonitoringHook tests monitoring callback
func TestLifecycleManager_MonitoringHook(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 50 * time.Millisecond,
	}

	manager := NewLifecycleManager(executor)

	// Track hook calls
	var hookCalls []string
	hook := func(subagentID string, status domain.SubagentStatus) {
		hookCalls = append(hookCalls, subagentID+":"+string(status))
	}

	manager.SetMonitoringHook(hook)

	subagentCtx := domain.SubagentContext{
		ID:              "test-hook",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	err := manager.Start(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Hook should have been called at least once
	if len(hookCalls) == 0 {
		t.Error("Monitoring hook was not called")
	}

	// Should have called with test-hook ID
	found := false
	for _, call := range hookCalls {
		if call == "test-hook:running" || call == "test-hook:complete" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Hook calls = %v, expected test-hook:running or test-hook:complete", hookCalls)
	}
}

// TestLifecycleManager_Shutdown tests graceful shutdown
func TestLifecycleManager_Shutdown(t *testing.T) {
	executor := &mockExecutor{
		executeDelay: 1 * time.Second, // Long delay
	}

	manager := NewLifecycleManager(executor)

	ctx := context.Background()

	// Start a subagent
	subagentCtx := domain.SubagentContext{
		ID:              "test-shutdown",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	err := manager.Start(ctx, subagentCtx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = manager.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	// All subagents should be stopped
	running := manager.ListRunning(ctx)
	if len(running) != 0 {
		t.Errorf("After shutdown, running count = %d, want 0", len(running))
	}
}
