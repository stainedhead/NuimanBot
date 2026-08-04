package chat

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/security"
)

// TestResolveConfirmation_Approved_ReinvokesToolAndReturnsReply covers P5.9's
// documented recommendation: ChatService.ResolveConfirmation(ctx, id, true)
// looks up the confirmation by ID (no live incoming chat message needed,
// unlike the gateway yes/no reply path), re-invokes the original tool call
// directly with its original params, and runs a fresh tool-loop turn,
// exactly like resolveConfirmationApproved.
func TestResolveConfirmation_Approved_ReinvokesToolAndReturnsReply(t *testing.T) {
	llmCallCount := 0
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			return &domain.LLMResponse{Content: "Done, merged PR #42.", FinishReason: "end_turn"}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	executeCalls := 0
	var executedToolName string
	var executedParams map[string]any
	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) { return []domain.Tool{}, nil },
		executeFunc: func(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
			executeCalls++
			executedToolName = toolName
			executedParams = params
			return &domain.ExecutionResult{Output: "PR #42 merged"}, nil
		},
	}

	resolveCalls := 0
	confirmationStore := &mockConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			if id != "conf-42" {
				t.Errorf("expected Get(conf-42), got Get(%s)", id)
			}
			return security.ConfirmationRequest{
				ID:             "conf-42",
				UserID:         "user-1",
				ConversationID: "cli:user-1",
				ToolName:       "github",
				Action:         "pr_merge",
				Params:         map[string]any{"action": "pr_merge", "pr_number": 42},
				Summary:        "Merge PR #42?",
				Status:         security.ConfirmationStatusPending,
			}, nil
		},
		resolveFunc: func(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
			resolveCalls++
			if id != "conf-42" || !approved {
				t.Errorf("expected Resolve(conf-42, true), got Resolve(%s, %v)", id, approved)
			}
			return security.ConfirmationRequest{
				ID:             "conf-42",
				ConversationID: "cli:user-1",
				ToolName:       "github",
				Params:         map[string]any{"action": "pr_merge", "pr_number": 42},
				Status:         security.ConfirmationStatusApproved,
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	outgoing, err := service.ResolveConfirmation(context.Background(), "conf-42", true)
	if err != nil {
		t.Fatalf("ResolveConfirmation failed: %v", err)
	}

	if resolveCalls != 1 {
		t.Errorf("expected exactly 1 Resolve call, got %d", resolveCalls)
	}
	if executeCalls != 1 {
		t.Errorf("expected exactly 1 direct tool re-invocation, got %d", executeCalls)
	}
	if executedToolName != "github" {
		t.Errorf("expected the original tool (github) to be re-invoked, got %q", executedToolName)
	}
	if executedParams["pr_number"] != 42 {
		t.Errorf("expected the original params to be reused verbatim, got %v", executedParams)
	}
	if llmCallCount != 1 {
		t.Errorf("expected exactly 1 fresh LLM call for the follow-up response, got %d", llmCallCount)
	}
	if outgoing.Content != "Done, merged PR #42." {
		t.Errorf("expected the fresh tool-loop's response, got %q", outgoing.Content)
	}
}

// TestResolveConfirmation_Denied_CancelsWithoutExecuting covers
// ResolveConfirmation(ctx, id, false): the confirmation is resolved as
// denied and the original tool call is never executed.
func TestResolveConfirmation_Denied_CancelsWithoutExecuting(t *testing.T) {
	llmCallCount := 0
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			return &domain.LLMResponse{Content: "should not be called"}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{}

	executeCalls := 0
	toolExecService := &mockToolExecutionService{
		executeFunc: func(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
			executeCalls++
			return &domain.ExecutionResult{Output: "should not run"}, nil
		},
	}

	resolveCalls := 0
	confirmationStore := &mockConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return security.ConfirmationRequest{
				ID:             "conf-99",
				UserID:         "user-1",
				ConversationID: "cli:user-1",
				Summary:        "Merge PR #99?",
				Status:         security.ConfirmationStatusPending,
			}, nil
		},
		resolveFunc: func(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
			resolveCalls++
			if id != "conf-99" || approved {
				t.Errorf("expected Resolve(conf-99, false), got Resolve(%s, %v)", id, approved)
			}
			return security.ConfirmationRequest{ID: "conf-99", Status: security.ConfirmationStatusDenied}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	outgoing, err := service.ResolveConfirmation(context.Background(), "conf-99", false)
	if err != nil {
		t.Fatalf("ResolveConfirmation failed: %v", err)
	}

	if resolveCalls != 1 {
		t.Errorf("expected exactly 1 Resolve call, got %d", resolveCalls)
	}
	if executeCalls != 0 {
		t.Errorf("expected the tool to never execute on denial, got %d calls", executeCalls)
	}
	if llmCallCount != 0 {
		t.Errorf("expected no LLM call on denial, got %d", llmCallCount)
	}
	if outgoing.Content != "Cancelled: Merge PR #99?" {
		t.Errorf("expected a cancellation message, got %q", outgoing.Content)
	}
}

// TestResolveConfirmation_UnknownID_ReturnsNotFoundError verifies
// ResolveConfirmation propagates security.ErrConfirmationNotFound unwrapped
// (via errors.Is) so REST callers can map it to a 404 response.
func TestResolveConfirmation_UnknownID_ReturnsNotFoundError(t *testing.T) {
	confirmationStore := &mockConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
		},
	}

	service := createTestService(&mockLLMService{}, &mockMemoryRepository{}, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	_, err := service.ResolveConfirmation(context.Background(), "does-not-exist", true)
	if !errors.Is(err, security.ErrConfirmationNotFound) {
		t.Fatalf("expected security.ErrConfirmationNotFound, got %v", err)
	}
}

// TestResolveConfirmation_NoStoreConfigured_ReturnsError verifies
// ResolveConfirmation fails closed (rather than panicking) when the Service
// has no ConfirmationStore configured.
func TestResolveConfirmation_NoStoreConfigured_ReturnsError(t *testing.T) {
	service := createTestService(&mockLLMService{}, &mockMemoryRepository{}, &mockToolExecutionService{}, &mockSecurityService{})

	_, err := service.ResolveConfirmation(context.Background(), "conf-1", true)
	if err == nil {
		t.Fatal("expected an error when no ConfirmationStore is configured, got nil")
	}
}

// TestResolveConfirmation_AlreadyResolved_ReturnsError verifies that
// resolving an already-approved/denied confirmation propagates
// security.ErrConfirmationAlreadyResolved unwrapped.
func TestResolveConfirmation_AlreadyResolved_ReturnsError(t *testing.T) {
	confirmationStore := &mockConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return security.ConfirmationRequest{
				ID:             "conf-5",
				UserID:         "user-1",
				ConversationID: "cli:user-1",
				Status:         security.ConfirmationStatusApproved,
			}, nil
		},
		resolveFunc: func(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
			return security.ConfirmationRequest{}, security.ErrConfirmationAlreadyResolved
		},
	}

	service := createTestService(&mockLLMService{}, &mockMemoryRepository{}, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	_, err := service.ResolveConfirmation(context.Background(), "conf-5", true)
	if !errors.Is(err, security.ErrConfirmationAlreadyResolved) {
		t.Fatalf("expected security.ErrConfirmationAlreadyResolved, got %v", err)
	}
}
