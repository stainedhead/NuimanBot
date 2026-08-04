package main

// Tests for wireConfirmationStore (specs/260803-improve-nuimanbot-security-
// auto-review, FR-014/P3.4 — see cmd/nuimanbot/main.go's doc comment on
// wireConfirmationStore for the full rationale).
//
// wireConfirmationStore is a structural fix: it is the ONLY place that calls
// chat.Service.SetConfirmationStore / tool.Service.SetConfirmationStore, and
// it takes a single `store` parameter that both calls use — so there is no
// way, at the call site in main.go, to pass the two services different
// ConfirmationStore instances. That guarantee is enforced by the Go
// compiler/the function's shape, not by any runtime check, so it cannot be
// unit-tested by "assert the two fields are equal" (both services keep
// their confirmationStore field unexported, and deliberately so — see
// AGENTS.md's Clean Architecture layering; cmd/nuimanbot must not reach into
// usecase-package internals).
//
// What CAN and MUST be tested at runtime is the fail-fast behavior: a nil
// store (or a nil service) must produce a clear error rather than silently
// leaving one or both services unconfigured. TestWireConfirmationStore_EndToEndSharedStore
// additionally proves, behaviorally and through exported methods only, that
// a confirmation created via tool.Service is resolvable via chat.Service —
// which is only possible if both ended up wired to the literal same store
// instance.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	infrasecurity "nuimanbot/internal/infrastructure/security"
	"nuimanbot/internal/usecase/chat"
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/tool"
)

// --- minimal fakes satisfying chat.Service's and tool.Service's constructor
// dependencies. None of these need real behavior for these tests: they only
// need to not panic on the handful of calls each test path makes. ---

type fakeChatLLMService struct{}

func (fakeChatLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	return &domain.LLMResponse{}, nil
}

func (fakeChatLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	return nil, nil
}

func (fakeChatLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	return nil, nil
}

type fakeChatMemoryRepo struct{}

func (fakeChatMemoryRepo) SaveMessage(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error {
	return nil
}

func (fakeChatMemoryRepo) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	return nil, nil
}

func (fakeChatMemoryRepo) GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
	return nil, nil
}

func (fakeChatMemoryRepo) DeleteConversation(ctx context.Context, convID string) error { return nil }

func (fakeChatMemoryRepo) ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error) {
	return nil, nil
}

type fakeChatToolExecService struct{}

func (fakeChatToolExecService) Execute(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
	return &domain.ExecutionResult{}, nil
}

func (fakeChatToolExecService) ExecuteWithUser(ctx context.Context, user *domain.User, conversationID, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
	return &domain.ExecutionResult{}, nil
}

func (fakeChatToolExecService) ListTools(ctx context.Context, user *domain.User) ([]domain.Tool, error) {
	return nil, nil
}

// fakeChatUserService satisfies chat.UserService, resolving every platform
// identity to a fixed RoleAdmin user — sufficient for this file's tests,
// which exercise ConfirmationStore wiring, not RBAC decisions.
type fakeChatUserService struct{}

func (fakeChatUserService) GetUserByPlatformUID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	return &domain.User{ID: platformUID, Role: domain.RoleAdmin}, nil
}

func (fakeChatUserService) CreateUser(ctx context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error) {
	return &domain.User{ID: platformUID, Role: role}, nil
}

// fakeSecurityService satisfies both chat.SecurityService (a subset) and
// domain.SecurityService (the full interface tool.NewService requires).
type fakeSecurityService struct{}

func (fakeSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (fakeSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (fakeSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	return input, nil
}

func (fakeSecurityService) GenerateAPIKey(ctx context.Context) (string, error) {
	return "test-key", nil
}

func (fakeSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error { return nil }

// newTestServices builds a real chat.Service and tool.Service pair, wired
// with the fakes above, exactly as main() does before calling
// wireConfirmationStore.
func newTestServices() (*chat.Service, *tool.Service) {
	toolSvc := tool.NewService(&config.ToolsSystemConfig{}, tool.NewInMemoryRegistry(), fakeSecurityService{})
	chatSvc := chat.NewService(fakeChatLLMService{}, fakeChatMemoryRepo{}, fakeChatToolExecService{}, fakeSecurityService{}, fakeChatUserService{})
	return chatSvc, toolSvc
}

// newTestConfirmationStore returns a real FileConfirmationStore rooted in a
// per-test temp directory (t.TempDir()), so tests exercise the exact same
// ConfirmationStore implementation production wiring uses.
func newTestConfirmationStore(t *testing.T) security.ConfirmationStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "confirmations.json")
	return infrasecurity.NewFileConfirmationStore(path, time.Minute)
}

// TestWireConfirmationStore_NilStoreFailsFast is the mandatory test from
// specs/260803-improve-nuimanbot-security-auto-review/tasks.md P3.4: a nil
// store must produce a clear, fail-fast error rather than silently wiring
// zero, one, or two services with no confirmation gating.
func TestWireConfirmationStore_NilStoreFailsFast(t *testing.T) {
	chatSvc, toolSvc := newTestServices()

	err := wireConfirmationStore(chatSvc, toolSvc, nil)
	if err == nil {
		t.Fatal("wireConfirmationStore(nil store): expected a fail-fast error, got nil")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		t.Errorf("wireConfirmationStore(nil store) error = %q, want a clear message about the nil store", err.Error())
	}
}

// TestWireConfirmationStore_NilChatServiceFailsFast covers the "one of the
// two services is missing" half of the finding: a nil chat.Service must
// error rather than silently wiring only tool.Service.
func TestWireConfirmationStore_NilChatServiceFailsFast(t *testing.T) {
	_, toolSvc := newTestServices()
	store := newTestConfirmationStore(t)

	err := wireConfirmationStore(nil, toolSvc, store)
	if err == nil {
		t.Fatal("wireConfirmationStore(nil chat.Service): expected a fail-fast error, got nil")
	}
}

// TestWireConfirmationStore_NilToolServiceFailsFast is the mirror image: a
// nil tool.Service must error rather than silently wiring only chat.Service.
func TestWireConfirmationStore_NilToolServiceFailsFast(t *testing.T) {
	chatSvc, _ := newTestServices()
	store := newTestConfirmationStore(t)

	err := wireConfirmationStore(chatSvc, nil, store)
	if err == nil {
		t.Fatal("wireConfirmationStore(nil tool.Service): expected a fail-fast error, got nil")
	}
}

// TestWireConfirmationStore_ValidInputsSucceed is the straightforward
// green-path counterpart: fully-formed, non-nil arguments must wire cleanly
// with no error.
func TestWireConfirmationStore_ValidInputsSucceed(t *testing.T) {
	chatSvc, toolSvc := newTestServices()
	store := newTestConfirmationStore(t)

	if err := wireConfirmationStore(chatSvc, toolSvc, store); err != nil {
		t.Fatalf("wireConfirmationStore with valid, non-nil arguments: unexpected error: %v", err)
	}
}

// TestWireConfirmationStore_EndToEndSharedStore is the behavioral proof that
// wireConfirmationStore wires both services to the SAME store instance —
// exactly the FR-014 finding's concern. Neither chat.Service nor
// tool.Service exposes its confirmationStore field (by design — see
// AGENTS.md's Clean Architecture layering, and cmd/nuimanbot deliberately
// staying out of usecase-package internals), so this test proves sharing
// through observable, exported behavior instead of field inspection:
//
//  1. toolSvc.Execute (with a security.ConfirmationIdentity attached, and
//     confirmationCfg configured to require confirmation for our test tool)
//     creates a pending confirmation in whatever store toolSvc was wired to.
//  2. chatSvc.ResolveConfirmation (deny) is then asked to resolve that exact
//     confirmation ID.
//
// If wireConfirmationStore had wired the two services to different store
// instances (or left chatSvc's store nil), step 2 would fail — the
// confirmation created in step 1 would not exist in chatSvc's store. Success
// here is only possible because both services observe the identical
// instance passed into wireConfirmationStore.
func TestWireConfirmationStore_EndToEndSharedStore(t *testing.T) {
	chatSvc, toolSvc := newTestServices()
	store := newTestConfirmationStore(t)

	if err := wireConfirmationStore(chatSvc, toolSvc, store); err != nil {
		t.Fatalf("wireConfirmationStore: unexpected error: %v", err)
	}

	toolSvc.SetConfirmationConfig(config.ConfirmationConfig{
		DefaultRequiredActions: []string{"test_tool"},
	})

	const userID = "user-1"
	const conversationID = "conv-1"
	ctx := security.WithConfirmationIdentity(context.Background(), userID, conversationID)

	result, err := toolSvc.Execute(ctx, "test_tool", map[string]any{"arg": "value"})
	if err != nil {
		t.Fatalf("toolSvc.Execute: unexpected error: %v", err)
	}
	if result.Status != domain.StatusPendingConfirmation {
		t.Fatalf("toolSvc.Execute: status = %q, want %q (was the confirmation gate actually triggered?)", result.Status, domain.StatusPendingConfirmation)
	}
	if result.ConfirmationID == "" {
		t.Fatal("toolSvc.Execute: expected a non-empty ConfirmationID for a pending confirmation")
	}

	// Resolve the confirmation tool.Service created, via chat.Service. This
	// only succeeds if chatSvc's confirmationStore is the exact same
	// instance toolSvc created the confirmation in.
	outgoing, err := chatSvc.ResolveConfirmation(context.Background(), result.ConfirmationID, false)
	if err != nil {
		t.Fatalf("chatSvc.ResolveConfirmation: unexpected error (indicates chat.Service and tool.Service are NOT sharing the same ConfirmationStore instance): %v", err)
	}
	if !strings.Contains(outgoing.Content, "Cancelled") {
		t.Errorf("chatSvc.ResolveConfirmation outgoing content = %q, want it to report cancellation", outgoing.Content)
	}
}
