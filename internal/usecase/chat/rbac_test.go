package chat

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/ratelimit"
	"nuimanbot/internal/usecase/tool"
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

// TestProcessMessage_RBACEnforcedAcrossPlatforms is the P3.1/P3.2 regression
// test: for Telegram, Slack, and CLI, a message that triggers a tool call
// must route through tool.Service.ExecuteWithUser (not the unchecked
// Execute), so a RoleGuest caller is denied a RoleUser-gated tool — and that
// tool must not even be listed for the LLM (FR-012, ListTools role
// filtering). Before this fix, Execute() silently succeeded regardless of
// role and ListTools ignored role entirely. Buzz's equivalent case is added
// in P3.3, once P3.1/P3.2 have both landed.
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
