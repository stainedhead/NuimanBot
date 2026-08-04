package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/ratelimit"
	"nuimanbot/internal/usecase/tool"
	"nuimanbot/internal/usecase/tool/github"
)

// rbacTestTool is a minimal domain.Tool that records whether it was actually
// invoked, so RBAC regression tests can assert a denied call never reaches
// tool execution (not just that ProcessMessage returned no error).
type rbacTestTool struct {
	name    string
	invoked *bool
}

func (t *rbacTestTool) Name() string        { return t.name }
func (t *rbacTestTool) Description() string { return "RBAC regression test tool" }
func (t *rbacTestTool) InputSchema() map[string]any {
	return map[string]any{"type": "object"}
}
func (t *rbacTestTool) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	*t.invoked = true
	return &domain.ExecutionResult{Output: "sunny"}, nil
}
func (t *rbacTestTool) RequiredPermissions() []domain.Permission { return nil }
func (t *rbacTestTool) Config() domain.ToolConfig                { return domain.ToolConfig{Enabled: true} }

// newRealToolService wires the actual tool.Service (not a mock) with a real
// rate limiter, so these regression tests genuinely exercise checkPermission,
// rate limiting, and audit logging end-to-end (FR-011), rather than trusting
// a test double to simulate them.
func newRealToolService(invoked *bool, auditEvents *[]domain.AuditEvent, rateLimit ratelimit.RateLimit) *tool.Service {
	registry := tool.NewInMemoryRegistry()
	_ = registry.Register(&rbacTestTool{name: "weather", invoked: invoked}) // "weather" requires RoleUser per tool.ToolPermissions

	securitySvc := &mockSecurityService{
		auditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			*auditEvents = append(*auditEvents, *event)
			return nil
		},
	}

	svc := tool.NewService(&config.ToolsSystemConfig{}, registry, securitySvc)
	svc.SetRateLimiter(ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{"default": rateLimit}))
	return svc
}

// toolCallThenFinalLLM returns a mockLLMService that requests the "weather"
// tool call on its first invocation and returns a plain final response on
// every subsequent call (ChatService's tool-calling loop always re-queries
// the LLM after executing/denying a tool call). If capturedTools is non-nil,
// the tools list from the first request is captured into it, so callers can
// assert on ListTools' role-filtered output (FR-012).
func toolCallThenFinalLLM(capturedTools *[]domain.ToolDefinition) *mockLLMService {
	callCount := 0
	return &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				if capturedTools != nil {
					*capturedTools = req.Tools
				}
				return &domain.LLMResponse{
					Content: "checking weather",
					ToolCalls: []domain.ToolCall{
						{ToolName: "weather", Arguments: map[string]any{"city": "NYC"}},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
				}, nil
			}
			return &domain.LLMResponse{
				Content:      "done",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
			}, nil
		},
	}
}

func hasAuditAction(events []domain.AuditEvent, action string) bool {
	for _, e := range events {
		if e.Action == action {
			return true
		}
	}
	return false
}

func hasToolNamed(tools []domain.ToolDefinition, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

// TestProcessMessage_RBACEnforcedAcrossPlatforms is the P3.1/P3.2/P3.3
// regression test: for every platform (Telegram, Slack, CLI, and Buzz), a
// message that triggers a tool call must route through
// tool.Service.ExecuteWithUser (not the unchecked Execute), so a RoleGuest
// caller is denied a RoleUser-gated tool — and that tool must not even be
// listed for the LLM (FR-012, ListTools role filtering). Before this fix,
// Execute() silently succeeded regardless of role and ListTools ignored role
// entirely. The Buzz cases require zero Buzz-specific code beyond what P3.1/
// P3.2 already added to ChatService — confirming the fix is genuinely
// platform-agnostic, not Buzz-specific with incidental coverage elsewhere.
func TestProcessMessage_RBACEnforcedAcrossPlatforms(t *testing.T) {
	cases := []struct {
		name        string
		platform    domain.Platform
		platformUID string
		role        domain.Role
		wantDenied  bool
	}{
		{"Telegram guest denied", domain.PlatformTelegram, "tg-guest", domain.RoleGuest, true},
		{"Telegram user allowed", domain.PlatformTelegram, "tg-user", domain.RoleUser, false},
		{"Slack guest denied", domain.PlatformSlack, "slack-guest", domain.RoleGuest, true},
		{"Slack user allowed", domain.PlatformSlack, "slack-user", domain.RoleUser, false},
		{"CLI guest denied", domain.PlatformCLI, "cli-guest", domain.RoleGuest, true},
		{"CLI user allowed", domain.PlatformCLI, "cli-user", domain.RoleUser, false},
		{"Buzz guest denied", domain.PlatformBuzz, "npub-guest", domain.RoleGuest, true},
		{"Buzz user allowed", domain.PlatformBuzz, "npub-user", domain.RoleUser, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			invoked := false
			var auditEvents []domain.AuditEvent
			toolExecService := newRealToolService(&invoked, &auditEvents, ratelimit.RateLimit{Requests: 100, Window: time.Minute})

			var capturedTools []domain.ToolDefinition
			llmService := toolCallThenFinalLLM(&capturedTools)
			memoryRepo := &mockMemoryRepository{}
			securityService := &mockSecurityService{}
			userService := &mockUserService{
				getUserFunc: func(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
					return &domain.User{
						ID:          tc.platformUID,
						Role:        tc.role,
						PlatformIDs: map[domain.Platform]string{platform: platformUID},
					}, nil
				},
			}

			service := NewService(llmService, memoryRepo, toolExecService, securityService, userService)

			incomingMsg := &domain.IncomingMessage{
				ID:          "rbac-test-" + tc.name,
				Platform:    tc.platform,
				PlatformUID: tc.platformUID,
				Text:        "what's the weather",
				Timestamp:   time.Now(),
			}

			if _, err := service.ProcessMessage(context.Background(), incomingMsg); err != nil {
				t.Fatalf("ProcessMessage failed: %v", err)
			}

			if hasToolNamed(capturedTools, "weather") == tc.wantDenied {
				t.Errorf("weather tool presence in LLM tool list = %v, want presence = %v (role %s)",
					hasToolNamed(capturedTools, "weather"), !tc.wantDenied, tc.role)
			}

			if tc.wantDenied {
				if invoked {
					t.Errorf("expected tool NOT to be invoked for denied role %s, but it was", tc.role)
				}
				if !hasAuditAction(auditEvents, "tool_execution_denied") {
					t.Errorf("expected a tool_execution_denied audit event, got: %+v", auditEvents)
				}
			} else {
				if !invoked {
					t.Errorf("expected tool to be invoked for allowed role %s, but it wasn't", tc.role)
				}
			}
		})
	}
}

// TestDefaultRoleForPlatform is a direct unit test of the platform-conditional
// default role: CLI is trusted/local (whoever runs the binary already has
// full machine access), so it defaults newly-created users to RoleAdmin,
// preserving the CLI's pre-existing de facto unrestricted access. Every other
// platform defaults to RoleGuest, since its sender is remote and
// unauthenticated by default.
func TestDefaultRoleForPlatform(t *testing.T) {
	cases := []struct {
		platform domain.Platform
		want     domain.Role
	}{
		{domain.PlatformCLI, domain.RoleAdmin},
		{domain.PlatformTelegram, domain.RoleGuest},
		{domain.PlatformSlack, domain.RoleGuest},
		{domain.PlatformBuzz, domain.RoleGuest},
	}
	for _, tc := range cases {
		if got := defaultRoleForPlatform(tc.platform); got != tc.want {
			t.Errorf("defaultRoleForPlatform(%s) = %s, want %s", tc.platform, got, tc.want)
		}
	}
}

// TestProcessMessage_CLIDefaultsToAdminOtherPlatformsToGuest is an
// integration-level regression test for the same platform-conditional
// default, exercised through the full resolveUser -> CreateUser -> RBAC
// path: a brand-new CLI user must be able to invoke a RoleUser-gated tool
// (created as RoleAdmin), while a brand-new Telegram/Slack/Buzz user must be
// denied the identical tool (created as RoleGuest).
func TestProcessMessage_CLIDefaultsToAdminOtherPlatformsToGuest(t *testing.T) {
	cases := []struct {
		name       string
		platform   domain.Platform
		wantDenied bool
	}{
		{"CLI fresh user defaults to RoleAdmin, tool allowed", domain.PlatformCLI, false},
		{"Telegram fresh user defaults to RoleGuest, tool denied", domain.PlatformTelegram, true},
		{"Slack fresh user defaults to RoleGuest, tool denied", domain.PlatformSlack, true},
		{"Buzz fresh user defaults to RoleGuest, tool denied", domain.PlatformBuzz, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			invoked := false
			var auditEvents []domain.AuditEvent
			toolExecService := newRealToolService(&invoked, &auditEvents, ratelimit.RateLimit{Requests: 100, Window: time.Minute})

			llmService := toolCallThenFinalLLM(nil)
			memoryRepo := &mockMemoryRepository{}
			securityService := &mockSecurityService{}
			// getUserFunc always reports not-found, forcing resolveUser down
			// the CreateUser(defaultRoleForPlatform(...)) path — the mock's
			// default createUserFunc echoes back whatever role it's given,
			// so the resulting role comes purely from production code.
			userService := &mockUserService{
				getUserFunc: func(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
					return nil, domain.ErrUserNotFound
				},
			}

			service := NewService(llmService, memoryRepo, toolExecService, securityService, userService)

			incomingMsg := &domain.IncomingMessage{
				ID:          "default-role-test-" + tc.name,
				Platform:    tc.platform,
				PlatformUID: "fresh-user",
				Text:        "what's the weather",
				Timestamp:   time.Now(),
			}

			if _, err := service.ProcessMessage(context.Background(), incomingMsg); err != nil {
				t.Fatalf("ProcessMessage failed: %v", err)
			}

			if tc.wantDenied {
				if invoked {
					t.Errorf("expected tool NOT to be invoked for a fresh %s user, but it was", tc.platform)
				}
			} else {
				if !invoked {
					t.Errorf("expected tool to be invoked for a fresh %s user, but it wasn't", tc.platform)
				}
			}
		})
	}
}

// TestProcessMessage_RateLimitEnforced verifies tool calls triggered from
// chat messages are genuinely rate-limited (part of FR-011's "checkPermission,
// rate limiting, and audit logging all exercised" requirement), not just
// permission-checked.
func TestProcessMessage_RateLimitEnforced(t *testing.T) {
	invoked := false
	var auditEvents []domain.AuditEvent
	// Capacity of 1: the first ProcessMessage call consumes the only token;
	// the second must be denied by the rate limiter, not by checkPermission.
	// The rate limiter's token bucket lives inside toolExecService, so it must
	// be shared across both calls below even though each gets its own
	// ChatService/LLM mock (a fresh mockLLMService per call, since its
	// call-count-based tool-call trigger must restart for each message).
	toolExecService := newRealToolService(&invoked, &auditEvents, ratelimit.RateLimit{Requests: 1, Window: time.Hour})

	memoryRepo := &mockMemoryRepository{}
	securityService := &mockSecurityService{}
	userService := &mockUserService{
		getUserFunc: func(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
			return &domain.User{ID: "rl-user", Role: domain.RoleUser, PlatformIDs: map[domain.Platform]string{platform: platformUID}}, nil
		},
	}

	makeMsg := func(id string) *domain.IncomingMessage {
		return &domain.IncomingMessage{
			ID:          id,
			Platform:    domain.PlatformCLI,
			PlatformUID: "rl-user",
			Text:        "what's the weather",
			Timestamp:   time.Now(),
		}
	}

	service1 := NewService(toolCallThenFinalLLM(nil), memoryRepo, toolExecService, securityService, userService)
	if _, err := service1.ProcessMessage(context.Background(), makeMsg("rl-1")); err != nil {
		t.Fatalf("first ProcessMessage failed: %v", err)
	}
	if !invoked {
		t.Fatal("expected first call to invoke the tool (within rate limit)")
	}

	invoked = false
	service2 := NewService(toolCallThenFinalLLM(nil), memoryRepo, toolExecService, securityService, userService)
	if _, err := service2.ProcessMessage(context.Background(), makeMsg("rl-2")); err != nil {
		t.Fatalf("second ProcessMessage failed: %v", err)
	}
	if invoked {
		t.Error("expected second call to be denied by the rate limiter, but tool was invoked")
	}
	if !hasAuditAction(auditEvents, "tool_rate_limit_exceeded") {
		t.Errorf("expected a tool_rate_limit_exceeded audit event, got: %+v", auditEvents)
	}
}

// newRealToolServiceWithGitHubTool builds a REAL tool.Service (not a mock)
// backed by an in-memory registry serving a single tool named "github" — the
// same name resolveRequiredRole's action-aware githubActionRole branch keys
// on (internal/usecase/tool/service.go) — so this test exercises the actual
// production RBAC decision tool.Service makes, not a mocked stand-in for it.
func newRealToolServiceWithGitHubTool(t *testing.T) *tool.Service {
	t.Helper()

	registry := tool.NewInMemoryRegistry()
	if err := registry.Register(&mockSkill{
		name:        "github",
		description: "GitHub tool (test double)",
		inputSchema: map[string]any{"type": "object"},
	}); err != nil {
		t.Fatalf("failed to register test github tool: %v", err)
	}

	return tool.NewService(&config.ToolsSystemConfig{}, registry, &mockSecurityService{})
}

// TestProcessMessage_RBAC_NonAdminCannotInvokeGitHubPRMerge is the
// FR-001/FR-002 regression test for the review's P0 finding: chat.Service's
// tool-calling loop called the RBAC-free ToolExecutionService.Execute, never
// ExecuteWithUser — making tool.Service's RBAC checks (admin-only
// "github"/"coding_agent" write actions) dead code from a live conversation's
// perspective. Before the fix, this test's non-admin chat user's message
// requesting github.pr_merge (an admin-only write action per
// internal/usecase/tool/permissions.go's githubActionRole) actually executes
// the tool. After the fix, chat.Service resolves a role-bearing domain.User
// for the incoming message's platform identity and calls ExecuteWithUser,
// which must deny this at the RBAC layer — surfacing a permission-denied
// tool result to the LLM, never a fabricated success.
func TestProcessMessage_RBAC_NonAdminCannotInvokeGitHubPRMerge(t *testing.T) {
	toolExecService := newRealToolServiceWithGitHubTool(t)

	llmCallCount := 0
	var secondCallLastMessage string

	llmService := &mockLLMService{
		completeFunc: func(_ context.Context, _ domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			if llmCallCount == 1 {
				return &domain.LLMResponse{
					Content: "Merging your PR now.",
					ToolCalls: []domain.ToolCall{
						{
							ToolName: "github",
							Arguments: map[string]any{
								"action": github.ActionPRMerge,
								"pr":     42.0,
							},
						},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
				}, nil
			}
			// Second call: capture the tool-result message the LLM actually
			// saw, so the test can assert on the real content rather than
			// just the final human-facing reply text.
			if len(req.Messages) > 0 {
				secondCallLastMessage = req.Messages[len(req.Messages)-1].Content
			}
			return &domain.LLMResponse{
				Content:      "I was not able to merge the PR.",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(_ context.Context, _ string, _ int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	securityService := &mockSecurityService{}

	userService := &mockUserService{
		getUserFunc: func(_ context.Context, _ domain.Platform, _ string) (*domain.User, error) {
			return &domain.User{ID: "U-non-admin-1", Role: domain.RoleUser}, nil // non-admin
		},
	}
	service := NewService(llmService, memoryRepo, toolExecService, securityService, userService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-rbac-1",
		Platform:    domain.PlatformSlack,
		PlatformUID: "U-non-admin-1",
		Text:        "Please merge PR 42",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage returned unexpected error: %v", err)
	}

	if llmCallCount != 2 {
		t.Fatalf("expected 2 LLM calls (tool round + final response), got %d", llmCallCount)
	}

	// The RBAC layer must have denied the call — proven by the tool-result
	// message fed back to the LLM carrying the permission-denied error, not
	// the mock tool's fabricated "mock result" success output.
	if strings.Contains(secondCallLastMessage, "mock result") {
		t.Fatalf("RBAC bypass: non-admin user's github.pr_merge call actually executed (tool result: %q)", secondCallLastMessage)
	}
	if !strings.Contains(secondCallLastMessage, domain.ErrInsufficientPermissions.Error()) {
		t.Fatalf("expected tool result to carry a permission-denied error, got: %q", secondCallLastMessage)
	}
}

// TestProcessMessage_RBAC_AdminCanInvokeGitHubPRMerge is the positive
// counterpart: an admin chat user's identical github.pr_merge request must
// still succeed once RBAC is actually enforced, proving the fix denies
// non-admins without breaking legitimate admin use.
func TestProcessMessage_RBAC_AdminCanInvokeGitHubPRMerge(t *testing.T) {
	toolExecService := newRealToolServiceWithGitHubTool(t)

	llmCallCount := 0
	var secondCallLastMessage string

	llmService := &mockLLMService{
		completeFunc: func(_ context.Context, _ domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			if llmCallCount == 1 {
				return &domain.LLMResponse{
					Content: "Merging your PR now.",
					ToolCalls: []domain.ToolCall{
						{
							ToolName: "github",
							Arguments: map[string]any{
								"action": github.ActionPRMerge,
								"pr":     42.0,
							},
						},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
				}, nil
			}
			if len(req.Messages) > 0 {
				secondCallLastMessage = req.Messages[len(req.Messages)-1].Content
			}
			return &domain.LLMResponse{
				Content:      "Merged.",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(_ context.Context, _ string, _ int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	securityService := &mockSecurityService{}

	userService := &mockUserService{
		getUserFunc: func(_ context.Context, _ domain.Platform, _ string) (*domain.User, error) {
			return &domain.User{ID: "U-admin-1", Role: domain.RoleAdmin}, nil // admin
		},
	}
	service := NewService(llmService, memoryRepo, toolExecService, securityService, userService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-rbac-2",
		Platform:    domain.PlatformSlack,
		PlatformUID: "U-admin-1",
		Text:        "Please merge PR 42",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage returned unexpected error: %v", err)
	}

	if strings.Contains(secondCallLastMessage, domain.ErrInsufficientPermissions.Error()) {
		t.Fatalf("expected admin's github.pr_merge call to succeed, but it was denied: %q", secondCallLastMessage)
	}
	if !strings.Contains(secondCallLastMessage, "mock result") {
		t.Fatalf("expected admin's github.pr_merge call to actually execute, got: %q", secondCallLastMessage)
	}
}

// TestProcessMessage_RBAC_UnresolvableIdentityFailsClosedToGuest verifies
// that when UserService has no existing record for a platform identity
// (domain.ErrUserNotFound), resolveUser's CreateUser fallback defaults a
// non-CLI platform to the lowest-privilege domain.RoleGuest
// (defaultRoleForPlatform) — denied for anything above RoleGuest — rather
// than silently granting any implicit trust to an unregistered identity.
func TestProcessMessage_RBAC_UnresolvableIdentityFailsClosedToGuest(t *testing.T) {
	toolExecService := newRealToolServiceWithGitHubTool(t)

	llmCallCount := 0
	var secondCallLastMessage string

	llmService := &mockLLMService{
		completeFunc: func(_ context.Context, _ domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			llmCallCount++
			if llmCallCount == 1 {
				return &domain.LLMResponse{
					Content: "Merging your PR now.",
					ToolCalls: []domain.ToolCall{
						{
							ToolName:  "github",
							Arguments: map[string]any{"action": github.ActionPRMerge, "pr": 42.0},
						},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
				}, nil
			}
			if len(req.Messages) > 0 {
				secondCallLastMessage = req.Messages[len(req.Messages)-1].Content
			}
			return &domain.LLMResponse{
				Content:      "I was not able to merge the PR.",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(_ context.Context, _ string, _ int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	userService := &mockUserService{
		getUserFunc: func(_ context.Context, _ domain.Platform, _ string) (*domain.User, error) {
			return nil, domain.ErrUserNotFound // unregistered — falls through to CreateUser
		},
	}
	service := NewService(llmService, memoryRepo, toolExecService, &mockSecurityService{}, userService)
	// defaultRoleForPlatform defaults new non-CLI users to RoleGuest (fail closed).

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-rbac-3",
		Platform:    domain.PlatformSlack,
		PlatformUID: "U-unregistered-1",
		Text:        "Please merge PR 42",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage returned unexpected error: %v", err)
	}

	if !strings.Contains(secondCallLastMessage, domain.ErrInsufficientPermissions.Error()) {
		t.Fatalf("expected unresolvable identity to fail closed (denied), got: %q", secondCallLastMessage)
	}
}
