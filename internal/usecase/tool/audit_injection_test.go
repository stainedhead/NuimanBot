package tool_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/security"
	. "nuimanbot/internal/usecase/tool"
)

// TestExecute_FlaggedSuccessResult_AuditIncludesInjectionFields verifies that when
// a tool succeeds but its ExecutionResult.Metadata carries injection_flagged (set
// by an OutputValidator-wired tool such as summarize/doc_summarize/websearch/mcp
// when action: annotate), the success audit event's Details map surfaces both
// injection_flagged and matched_patterns.
func TestExecute_FlaggedSuccessResult_AuditIncludesInjectionFields(t *testing.T) {
	mockResult := &domain.ExecutionResult{
		Output: security.AnnotateFlaggedContent("some content"),
		Metadata: map[string]any{
			"injection_flagged": true,
			"matched_patterns":  []string{"ignore previous instructions"},
		},
	}
	mockTool := &MockTool{
		NameFunc: func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return mockResult, nil
		},
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, err := svc.Execute(ctx, "testskill", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	<-auditEvents // discard "attempt" event

	select {
	case successEvent := <-auditEvents:
		if successEvent.Outcome != "success" {
			t.Fatalf("expected 'success' outcome, got %s", successEvent.Outcome)
		}
		flagged, ok := successEvent.Details["injection_flagged"].(bool)
		if !ok || !flagged {
			t.Errorf("expected Details[injection_flagged]=true, got %v", successEvent.Details["injection_flagged"])
		}
		patterns, ok := successEvent.Details["matched_patterns"].([]string)
		if !ok || len(patterns) == 0 {
			t.Errorf("expected Details[matched_patterns] to be a populated []string, got %v", successEvent.Details["matched_patterns"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for success audit event")
	}
}

// TestExecute_CleanSuccessResult_AuditOmitsInjectionFields verifies a clean tool
// call's audit event does not spuriously report injection flags.
func TestExecute_CleanSuccessResult_AuditOmitsInjectionFields(t *testing.T) {
	mockResult := &domain.ExecutionResult{Output: "clean output"}
	mockTool := &MockTool{
		NameFunc: func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return mockResult, nil
		},
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, err := svc.Execute(ctx, "testskill", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	<-auditEvents // discard "attempt" event

	select {
	case successEvent := <-auditEvents:
		if flagged, present := successEvent.Details["injection_flagged"]; present && flagged == true {
			t.Errorf("expected clean call to omit/zero-value injection_flagged, got %v", flagged)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for success audit event")
	}
}

// TestExecute_FlaggedOutputError_AuditIncludesInjectionFields verifies that when a
// tool fails closed with a *security.FlaggedOutputError (the reject action path),
// the failure audit event's Details map surfaces injection_flagged and
// matched_patterns recovered from the error via errors.As.
func TestExecute_FlaggedOutputError_AuditIncludesInjectionFields(t *testing.T) {
	flaggedErr := &security.FlaggedOutputError{
		Source:          "https://evil.example",
		MatchedPatterns: []string{"ignore previous instructions"},
	}
	mockTool := &MockTool{
		NameFunc: func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return nil, flaggedErr
		},
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, err := svc.Execute(ctx, "testskill", map[string]any{})
	if !errors.As(err, new(*security.FlaggedOutputError)) {
		t.Fatalf("expected returned error to wrap *security.FlaggedOutputError, got: %v", err)
	}

	<-auditEvents // discard "attempt" event

	select {
	case failureEvent := <-auditEvents:
		if failureEvent.Outcome != "failure" {
			t.Fatalf("expected 'failure' outcome, got %s", failureEvent.Outcome)
		}
		flagged, ok := failureEvent.Details["injection_flagged"].(bool)
		if !ok || !flagged {
			t.Errorf("expected Details[injection_flagged]=true, got %v", failureEvent.Details["injection_flagged"])
		}
		patterns, ok := failureEvent.Details["matched_patterns"].([]string)
		if !ok || len(patterns) == 0 {
			t.Errorf("expected Details[matched_patterns] to be a populated []string, got %v", failureEvent.Details["matched_patterns"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for failure audit event")
	}
}

// TestExecute_OrdinaryError_AuditOmitsInjectionFields verifies a plain (non-flagged)
// tool failure does not spuriously report injection flags.
func TestExecute_OrdinaryError_AuditOmitsInjectionFields(t *testing.T) {
	ordinaryErr := errors.New("some ordinary failure")
	mockTool := &MockTool{
		NameFunc: func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return nil, ordinaryErr
		},
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, _ = svc.Execute(ctx, "testskill", map[string]any{})

	<-auditEvents // discard "attempt" event

	select {
	case failureEvent := <-auditEvents:
		if flagged, present := failureEvent.Details["injection_flagged"]; present && flagged == true {
			t.Errorf("expected ordinary failure to omit/zero-value injection_flagged, got %v", flagged)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for failure audit event")
	}
}
