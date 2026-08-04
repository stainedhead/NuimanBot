package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/security"
)

// mockConfirmationStore implements security.ConfirmationStore for
// chat-package tests exercising P5.4 (tool-loop pending-confirmation
// turn-ending) and P5.5 (confirmation-reply detection at the start of
// ProcessMessage).
type mockConfirmationStore struct {
	createFunc       func(ctx context.Context, req security.ConfirmationRequest) (string, error)
	resolveFunc      func(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error)
	getFunc          func(ctx context.Context, id string) (security.ConfirmationRequest, error)
	getOpenByKeyFunc func(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error)
	expireStaleFunc  func(ctx context.Context) error
	listPendingFunc  func(ctx context.Context) ([]security.ConfirmationRequest, error)
}

func (m *mockConfirmationStore) Create(ctx context.Context, req security.ConfirmationRequest) (string, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return "", errors.New("Create not implemented in mock")
}

func (m *mockConfirmationStore) Resolve(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, id, approved)
	}
	return security.ConfirmationRequest{}, errors.New("Resolve not implemented in mock")
}

func (m *mockConfirmationStore) Get(ctx context.Context, id string) (security.ConfirmationRequest, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return security.ConfirmationRequest{}, errors.New("Get not implemented in mock")
}

func (m *mockConfirmationStore) GetOpenByKey(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
	if m.getOpenByKeyFunc != nil {
		return m.getOpenByKeyFunc(ctx, userID, conversationID)
	}
	return security.ConfirmationRequest{}, false, nil
}

func (m *mockConfirmationStore) ExpireStale(ctx context.Context) error {
	if m.expireStaleFunc != nil {
		return m.expireStaleFunc(ctx)
	}
	return nil
}

func (m *mockConfirmationStore) ListPending(ctx context.Context) ([]security.ConfirmationRequest, error) {
	if m.listPendingFunc != nil {
		return m.listPendingFunc(ctx)
	}
	return nil, nil
}

func toolCallingLLM(callCount *int, toolName string, args map[string]any) *mockLLMService {
	return &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			*callCount++
			return &domain.LLMResponse{
				Content:      "Let me do that.",
				ToolCalls:    []domain.ToolCall{{ToolName: toolName, Arguments: args}},
				FinishReason: "tool_use",
				Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}, nil
		},
	}
}

// TestProcessMessage_PendingConfirmation_EndsTurnImmediately covers P5.4: a
// tool call returning domain.StatusPendingConfirmation ends the current turn
// right away — the reply is the Summary, and Metadata carries the
// confirmation_id for gateway agents to use.
func TestProcessMessage_PendingConfirmation_EndsTurnImmediately(t *testing.T) {
	callCount := 0
	llmService := toolCallingLLM(&callCount, "github", map[string]any{"action": "pr_merge"})

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) { return []domain.Tool{}, nil },
		executeFunc: func(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{
				Status:         domain.StatusPendingConfirmation,
				ConfirmationID: "conf-123",
				Summary:        "Confirm github.pr_merge (pr_number=42)?",
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})

	incomingMsg := &domain.IncomingMessage{
		ID:          "msg-1",
		Platform:    domain.PlatformCLI,
		PlatformUID: "user-1",
		Text:        "Merge PR 42",
		Timestamp:   time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if outgoingMsg.Content != "Confirm github.pr_merge (pr_number=42)?" {
		t.Errorf("expected the reply to be the confirmation Summary, got %q", outgoingMsg.Content)
	}
	if outgoingMsg.Metadata["status"] != "pending_confirmation" {
		t.Errorf("expected Metadata[status]=pending_confirmation, got %v", outgoingMsg.Metadata["status"])
	}
	if outgoingMsg.Metadata["confirmation_id"] != "conf-123" {
		t.Errorf("expected Metadata[confirmation_id]=conf-123, got %v", outgoingMsg.Metadata["confirmation_id"])
	}
	if callCount != 1 {
		t.Errorf("expected exactly 1 LLM call (no further iteration after the pending confirmation), got %d", callCount)
	}
}

// TestProcessMessage_PendingConfirmation_OnLastIteration_DoesNotExceedMaxIterations
// covers the specific edge tasks.md calls out: a pending confirmation
// detected on the final allowed tool-loop iteration must still succeed, not
// be reported as "max tool calling iterations exceeded".
func TestProcessMessage_PendingConfirmation_OnLastIteration_DoesNotExceedMaxIterations(t *testing.T) {
	callCount := 0
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			return &domain.LLMResponse{
				Content:      "Working on it.",
				ToolCalls:    []domain.ToolCall{{ToolName: "github", Arguments: map[string]any{"action": "pr_merge"}}},
				FinishReason: "tool_use",
				Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	toolCallCount := 0
	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) { return []domain.Tool{}, nil },
		executeFunc: func(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
			toolCallCount++
			if toolCallCount < 5 {
				// First 4 iterations: an ordinary (non-pending) tool result,
				// so the loop keeps iterating.
				return &domain.ExecutionResult{Output: "still working"}, nil
			}
			// 5th call (the last allowed iteration): pending confirmation.
			return &domain.ExecutionResult{
				Status:         domain.StatusPendingConfirmation,
				ConfirmationID: "conf-last",
				Summary:        "Confirm on the last iteration?",
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})

	incomingMsg := &domain.IncomingMessage{
		ID: "msg-2", Platform: domain.PlatformCLI, PlatformUID: "user-1", Text: "Do the thing", Timestamp: time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("expected success even though the pending confirmation was detected on the last iteration, got error: %v", err)
	}
	if outgoingMsg.Content != "Confirm on the last iteration?" {
		t.Errorf("expected the confirmation Summary as the reply, got %q", outgoingMsg.Content)
	}
	if callCount != 5 {
		t.Errorf("expected exactly 5 LLM calls, got %d", callCount)
	}
}

// TestProcessMessage_ConfirmationReply_Approve_ReinvokesToolDirectly covers
// P5.5: a "yes" reply to an open confirmation resolves it as approved,
// re-invokes the original tool call directly (bypassing a fresh LLM
// decision), and feeds the result into a fresh tool-loop invocation.
func TestProcessMessage_ConfirmationReply_Approve_ReinvokesToolDirectly(t *testing.T) {
	llmCallCount := 0
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			return &domain.LLMResponse{
				Content:      "Done, merged PR #42.",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{CompletionTokens: 5},
			}, nil
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
		getOpenByKeyFunc: func(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
			return security.ConfirmationRequest{
				ID:             "conf-42",
				UserID:         userID,
				ConversationID: conversationID,
				ToolName:       "github",
				Action:         "pr_merge",
				Params:         map[string]any{"action": "pr_merge", "pr_number": 42},
				Summary:        "Merge PR #42?",
				Status:         security.ConfirmationStatusPending,
			}, true, nil
		},
		resolveFunc: func(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
			resolveCalls++
			if id != "conf-42" || !approved {
				t.Errorf("expected Resolve(conf-42, true), got Resolve(%s, %v)", id, approved)
			}
			return security.ConfirmationRequest{
				ID:       "conf-42",
				ToolName: "github",
				Params:   map[string]any{"action": "pr_merge", "pr_number": 42},
				Status:   security.ConfirmationStatusApproved,
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	incomingMsg := &domain.IncomingMessage{
		ID: "msg-3", Platform: domain.PlatformCLI, PlatformUID: "user-1", Text: "yes", Timestamp: time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
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
		t.Errorf("expected exactly 1 fresh LLM call for the follow-up response (no re-decision call), got %d", llmCallCount)
	}
	if outgoingMsg.Content != "Done, merged PR #42." {
		t.Errorf("expected the fresh tool-loop's response, got %q", outgoingMsg.Content)
	}
}

// TestProcessMessage_ConfirmationReply_Deny_CancelsWithoutExecuting covers
// P5.5's denial path: a "no" reply resolves the confirmation as denied and
// the original tool call is never executed.
func TestProcessMessage_ConfirmationReply_Deny_CancelsWithoutExecuting(t *testing.T) {
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
		getOpenByKeyFunc: func(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
			return security.ConfirmationRequest{
				ID:      "conf-99",
				Summary: "Merge PR #99?",
				Status:  security.ConfirmationStatusPending,
			}, true, nil
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

	incomingMsg := &domain.IncomingMessage{
		ID: "msg-4", Platform: domain.PlatformCLI, PlatformUID: "user-1", Text: "no", Timestamp: time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
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
	if outgoingMsg.Content != "Cancelled: Merge PR #99?" {
		t.Errorf("expected a cancellation message, got %q", outgoingMsg.Content)
	}
	if outgoingMsg.Metadata["status"] != "confirmation_denied" {
		t.Errorf("expected Metadata[status]=confirmation_denied, got %v", outgoingMsg.Metadata["status"])
	}
}

// TestProcessMessage_ConfirmationReply_Ambiguous_ProcessesAsNewTurn covers
// P5.5: a message that neither approves nor denies an open confirmation
// leaves it pending and is processed as a normal new turn.
func TestProcessMessage_ConfirmationReply_Ambiguous_ProcessesAsNewTurn(t *testing.T) {
	llmCallCount := 0
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			return &domain.LLMResponse{Content: "What's the weather like?", FinishReason: "end_turn"}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) { return []domain.Tool{}, nil },
	}

	resolveCalls := 0
	confirmationStore := &mockConfirmationStore{
		getOpenByKeyFunc: func(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
			return security.ConfirmationRequest{ID: "conf-1", Summary: "Merge?", Status: security.ConfirmationStatusPending}, true, nil
		},
		resolveFunc: func(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
			resolveCalls++
			return security.ConfirmationRequest{}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	incomingMsg := &domain.IncomingMessage{
		ID: "msg-5", Platform: domain.PlatformCLI, PlatformUID: "user-1", Text: "what's the weather", Timestamp: time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if resolveCalls != 0 {
		t.Errorf("expected the open confirmation to remain untouched (not resolved) for an ambiguous reply, got %d Resolve calls", resolveCalls)
	}
	if llmCallCount != 1 {
		t.Errorf("expected the message to be processed as a normal new turn (1 LLM call), got %d", llmCallCount)
	}
	if outgoingMsg.Content != "What's the weather like?" {
		t.Errorf("expected the normal-turn response, got %q", outgoingMsg.Content)
	}
}

// TestProcessMessage_NoOpenConfirmation_ProcessesNormally verifies that when
// a ConfirmationStore is configured but there's no open confirmation for
// this (user, conversation), ProcessMessage behaves exactly like the no-store
// case (normal turn processing, no special-casing).
func TestProcessMessage_NoOpenConfirmation_ProcessesNormally(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "Hi there!", FinishReason: "end_turn"}, nil
		},
	}
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}
	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) { return []domain.Tool{}, nil },
	}
	confirmationStore := &mockConfirmationStore{
		getOpenByKeyFunc: func(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
			return security.ConfirmationRequest{}, false, nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})
	service.SetConfirmationStore(confirmationStore)

	incomingMsg := &domain.IncomingMessage{ID: "msg-6", Platform: domain.PlatformCLI, PlatformUID: "user-1", Text: "hi", Timestamp: time.Now()}
	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}
	if outgoingMsg.Content != "Hi there!" {
		t.Errorf("expected normal turn processing, got %q", outgoingMsg.Content)
	}
}

// TestRunToolLoop_PendingConfirmation_RoundMateResultsNotDiscarded is the
// regression test for the review's P1 finding FR-010/FR-R10
// (specs/260803-improve-nuimanbot-security-auto-review): when a single round
// contains multiple tool calls and one of them returns a pending-confirmation
// status, any OTHER call in that same round that already completed
// (successfully, here) must still have its result recorded in
// collectedToolOutputs — only the loop's flow-continuation (not iterating
// further) may be gated by the pending confirmation, not the visibility of
// its already-completed round-mates.
//
// Before the fix: runToolLoop checked firstPendingConfirmation and returned
// immediately, before the loop that populates collectedToolOutputs ever ran —
// so the calculator call's already-completed "4" result was silently
// discarded even though it had nothing to do with the github call's pending
// confirmation.
func TestRunToolLoop_PendingConfirmation_RoundMateResultsNotDiscarded(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(_ context.Context, _ domain.LLMProvider, _ *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{
				Content: "Working on it.",
				ToolCalls: []domain.ToolCall{
					{ToolName: "calculator", Arguments: map[string]any{"expression": "2+2"}},
					{ToolName: "github", Arguments: map[string]any{"action": "pr_merge"}},
				},
				FinishReason: "tool_use",
				Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}, nil
		},
	}

	toolExecService := &mockToolExecutionService{
		executeWithUserFunc: func(_ context.Context, _ *domain.User, _, toolName string, _ map[string]any) (*domain.ExecutionResult, error) {
			switch toolName {
			case "calculator":
				return &domain.ExecutionResult{Output: "4"}, nil
			case "github":
				return &domain.ExecutionResult{
					Status:         domain.StatusPendingConfirmation,
					ConfirmationID: "conf-round",
					Summary:        "Confirm github.pr_merge?",
				}, nil
			default:
				return nil, fmt.Errorf("unexpected tool call in test: %s", toolName)
			}
		},
	}

	service := createTestService(llmService, &mockMemoryRepository{}, toolExecService, &mockSecurityService{})

	user := &domain.User{ID: "user-1", Role: domain.RoleUser}
	llmRequest := &domain.LLMRequest{
		Messages: []domain.Message{{Role: "user", Content: "add 2+2 and merge PR 42"}},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, collectedToolOutputs, pending, err := service.runToolLoop(context.Background(), user, "conv-1", llmRequest, logger)
	if err != nil {
		t.Fatalf("runToolLoop returned unexpected error: %v", err)
	}
	if pending == nil {
		t.Fatalf("expected a pending confirmation to be returned")
	}
	if pending.ID != "conf-round" {
		t.Errorf("expected pending confirmation ID conf-round, got %q", pending.ID)
	}

	found := false
	for _, out := range collectedToolOutputs {
		if strings.Contains(out, "calculator") && strings.Contains(out, "4") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the calculator tool's already-completed result to be recorded in collectedToolOutputs even though the round also produced a pending confirmation; got %v", collectedToolOutputs)
	}
}

// TestClassifyConfirmationReply covers the yes/no heuristic directly.
func TestClassifyConfirmationReply(t *testing.T) {
	cases := map[string]confirmationReplyResolution{
		"yes":        confirmationReplyApprove,
		"Yes":        confirmationReplyApprove,
		" YES  ":     confirmationReplyApprove,
		"y":          confirmationReplyApprove,
		"approve":    confirmationReplyApprove,
		"confirm":    confirmationReplyApprove,
		"no":         confirmationReplyDeny,
		"N":          confirmationReplyDeny,
		"deny":       confirmationReplyDeny,
		"cancel":     confirmationReplyDeny,
		"reject":     confirmationReplyDeny,
		"maybe":      confirmationReplyAmbiguous,
		"":           confirmationReplyAmbiguous,
		"yes please": confirmationReplyAmbiguous,
	}

	for input, want := range cases {
		if got := classifyConfirmationReply(input); got != want {
			t.Errorf("classifyConfirmationReply(%q) = %v, want %v", input, got, want)
		}
	}
}
