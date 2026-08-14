package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time" // For time.Now()

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/requestid"
	"nuimanbot/internal/usecase/security"
)

// LLMService defines the interface for LLM interactions required by the ChatService.
// This is effectively a subset or exact copy of domain.LLMService.
type LLMService interface {
	Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error)
	Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error)
	ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error)
}

// MemoryRepository defines the interface for memory persistence required by the ChatService.
// This is effectively a subset or exact copy of domain.MemoryRepository.
type MemoryRepository interface {
	SaveMessage(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error
	GetConversation(ctx context.Context, convID string) (*domain.Conversation, error)
	GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error)
	DeleteConversation(ctx context.Context, convID string) error
	ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error)
}

// ToolExecutionService defines the interface for tool execution required by the ChatService.
// ExecuteWithUser (not the unchecked Execute) is used so tool calls triggered
// from chat messages are RBAC- and rate-limit-checked for every platform (FR-011).
type ToolExecutionService interface {
	// Execute runs a tool with no RBAC/permission checks applied. Used only
	// by the confirmation-approval re-invocation path (see
	// resolveConfirmationApproved), which has already been approved and must
	// not be re-gated — every other call site must use ExecuteWithUser (see
	// specs/260803-improve-nuimanbot-security-auto-review's FR-001 fix).
	Execute(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error)
	// ExecuteWithUser runs a tool after applying RBAC (role/AllowedTools) and
	// Part C's confirmation gate. This is the entry point ChatService's
	// tool-calling loop must use for any tool call originating from live LLM
	// output (FR-001 fix). conversationID scopes Part C's confirmation gate.
	ExecuteWithUser(ctx context.Context, user *domain.User, conversationID, toolName string, params map[string]any) (*domain.ExecutionResult, error)
	// ListTools returns the tools user's role permits (FR-002 fix, FR-012).
	ListTools(ctx context.Context, user *domain.User) ([]domain.Tool, error)
}

// UserService defines the interface for resolving/creating the platform-scoped
// domain.User required to enforce RBAC on tool calls (FR-006, FR-011). This is
// the single platform-agnostic entry point used by ChatService for every
// gateway (Telegram, Slack, CLI, Buzz) — gateways are not required to resolve
// a user themselves before dispatching to ChatService.ProcessMessage.
type UserService interface {
	GetUserByPlatformUID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error)
	CreateUser(ctx context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error)
}

// SecurityService defines the interface for security operations required by the ChatService.
// This is effectively a subset or exact copy of domain.SecurityService.
type SecurityService interface {
	ValidateInput(ctx context.Context, input string, maxLength int) (string, error)
	Audit(ctx context.Context, event *domain.AuditEvent) error
	// Other methods from SecurityService, e.g., Encrypt/Decrypt if chat needs them
}

// LLMCache defines the interface for caching LLM responses.
type LLMCache interface {
	Get(ctx context.Context, prompt string) (*domain.LLMResponse, bool)
	Set(ctx context.Context, prompt string, response *domain.LLMResponse)
}

// MemoryCurator extracts memory cells from chat interactions for long-term storage.
type MemoryCurator interface {
	ExtractMemoryCells(ctx context.Context, conversationID, userMessage, assistantReply string, toolOutputs []string) error
}

// MemoryRecaller retrieves relevant memories for context injection.
type MemoryRecaller interface {
	RecallAndFormat(ctx context.Context, conversationID, query string, maxTokens int) (string, error)
}

// PromptComposer composes system prompts from persona files.
type PromptComposer interface {
	Compose(ctx context.Context, input PromptComposerInput) (*PromptComposerOutput, error)
}

// PromptComposerInput represents input for prompt composition.
type PromptComposerInput struct {
	UserID   string
	Platform string
}

// PromptComposerOutput represents composed system prompt.
type PromptComposerOutput struct {
	SystemPrompt   string
	TokensUsed     int
	Truncated      bool
	TruncatedFiles []string
}

// LLMDefaults holds default LLM parameters used when constructing LLM requests.
type LLMDefaults struct {
	Model       string  // Default model name (e.g., "anthropic/claude-sonnet")
	MaxTokens   int     // Default max tokens for completions
	Temperature float64 // Default temperature for completions
}

// Service implements the ChatService use case.
type Service struct {
	llmService      LLMService
	memoryRepo      MemoryRepository
	toolExecService ToolExecutionService
	securityService SecurityService
	userService     UserService    // Resolves/creates the domain.User used for RBAC on tool calls
	cache           LLMCache       // Optional cache for LLM responses
	memoryCurator   MemoryCurator  // Optional memory curator for long-term storage
	memoryRecaller  MemoryRecaller // Optional memory recaller for context injection
	promptComposer  PromptComposer // Optional persona-aware prompt composer
	llmDefaults     LLMDefaults    // Default LLM parameters from config

	// confirmationStore backs Part C's confirmation-reply detection
	// (specs/260802-improve-nuimanbot-security, FR-013): at the start of
	// every ProcessMessage call, it's used to check whether the incoming
	// message resolves an open confirmation for (PlatformUID, conversationID)
	// before the message is treated as a new chat turn. Optional — when nil,
	// ProcessMessage skips confirmation-reply detection entirely (every
	// message is processed as a new turn, matching pre-Phase-5 behavior).
	//
	// Independently of this field, ProcessMessage always attaches a
	// security.ConfirmationIdentity to the tool-loop's context (see
	// security.WithConfirmationIdentity) so that
	// ToolExecutionService.Execute can apply the confirmation *gate* even
	// when this specific ChatService instance has no store reference of its
	// own for reply *detection* — the two concerns are independent, and in
	// production DI wiring both are set to the same underlying store.
	confirmationStore security.ConfirmationStore
}

// NewService creates a new ChatService instance.
func NewService(
	llmService LLMService,
	memoryRepo MemoryRepository,
	toolExecService ToolExecutionService,
	securityService SecurityService,
	userService UserService,
) *Service {
	return &Service{
		llmService:      llmService,
		memoryRepo:      memoryRepo,
		toolExecService: toolExecService,
		securityService: securityService,
		userService:     userService,
	}
}

// SetCache sets the LLM response cache (optional).
func (s *Service) SetCache(cache LLMCache) {
	s.cache = cache
}

// SetMemoryCurator sets the memory curator for extracting long-term memories (optional).
func (s *Service) SetMemoryCurator(curator MemoryCurator) {
	s.memoryCurator = curator
}

// SetMemoryRecaller sets the memory recaller for context injection (optional).
func (s *Service) SetMemoryRecaller(recaller MemoryRecaller) {
	s.memoryRecaller = recaller
}

// SetPromptComposer sets the persona-aware prompt composer (optional).
func (s *Service) SetPromptComposer(composer PromptComposer) {
	s.promptComposer = composer
}

// SetLLMDefaults sets default LLM parameters from configuration.
func (s *Service) SetLLMDefaults(defaults LLMDefaults) {
	s.llmDefaults = defaults
}

// SetConfirmationStore sets the ConfirmationStore used for Part C's
// confirmation-reply detection (FR-013). Optional — see the confirmationStore
// field's doc comment for what happens when it's never called.
func (s *Service) SetConfirmationStore(store security.ConfirmationStore) {
	s.confirmationStore = store
}

// defaultModel returns the configured default model or a sensible fallback.
func (s *Service) defaultModel() string {
	if s.llmDefaults.Model != "" {
		return s.llmDefaults.Model
	}
	return "claude-3-sonnet-20240229"
}

// defaultMaxTokens returns the configured default max tokens or a sensible fallback.
func (s *Service) defaultMaxTokens() int {
	if s.llmDefaults.MaxTokens > 0 {
		return s.llmDefaults.MaxTokens
	}
	return 1024
}

// defaultTemperature returns the configured default temperature or a sensible fallback.
func (s *Service) defaultTemperature() float64 {
	if s.llmDefaults.Temperature > 0 {
		return s.llmDefaults.Temperature
	}
	return 0.7
}

// getConversationID generates a conversation ID based on platform and user
func getConversationID(platform domain.Platform, platformUID string) string {
	return string(platform) + ":" + platformUID
}

// resolveUser resolves or creates the domain.User for (platform, platformUID).
// This is the platform-agnostic RBAC entry point (FR-006, FR-011) used for
// every gateway's messages, not just Buzz's.
func (s *Service) resolveUser(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	user, err := s.userService.GetUserByPlatformUID(ctx, platform, platformUID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("failed to look up user: %w", err)
	}

	user, err = s.userService.CreateUser(ctx, platform, platformUID, defaultRoleForPlatform(platform))
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			// Lost a race with a concurrent first-message create for the same
			// brand-new platform UID; look up the winner instead of failing.
			return s.userService.GetUserByPlatformUID(ctx, platform, platformUID)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// defaultRoleForPlatform returns the role assigned to a newly-created user on
// first message from platform. CLI is deliberately special-cased to
// RoleAdmin rather than RoleGuest: it's inherently local/trusted — whoever
// can run the binary already has full machine access, so gating tool access
// behind RBAC there adds friction without adding real security. This
// preserves the CLI's pre-existing de facto unrestricted access. Every other
// platform (Telegram, Slack, Buzz) defaults to RoleGuest, since the sender is
// a remote, unauthenticated-by-default party.
//
// LANDMINE (documented explicitly by the CLI-parity auto-review fix pass,
// FR-004): the "inherently local/trusted" justification above holds ONLY
// for an unauthenticated placeholder identity — it does NOT hold for a
// real, logged-in CLI user, where "whoever can run the binary" and "the
// person this session is authenticated as" are no longer the same
// guarantee. This PlatformCLI branch is dead-in-practice-but-NOT-dead-in-
// code: it still runs, unchanged, for any CLI-originated message whose
// (platform, platformUID) hasn't already been reconciled to a real
// domain.User with the caller's actual auth.Service-verified role. Today
// that's neutralized entirely at a different layer —
// internal/adapter/gateway/cli.AuthCommandHandler.EnsureAuthenticated /
// reconcileIdentity runs before Gateway.Start's REPL loop ever accepts
// input, so resolveUser's auto-create path (which calls this function)
// never actually fires for an authenticated CLI identity (see ADR-020 /
// documentation/technical-details.md's "Identity Reconciliation (AD-6)"
// section). That is a wiring-order mitigation, not a fix to this function.
// ANY future CLI-originated message path — a new command source, a
// background job, a future socket-server mode (see this feature's
// Non-Goals) — that reaches ProcessMessage/resolveUser WITHOUT first going
// through EnsureAuthenticated/reconcileIdentity will silently re-arm this
// shortcut and grant that caller RoleAdmin on first contact. Route any new
// CLI entry point through the auth gate first, or fix this function
// structurally (see ADR-020's optional follow-up) — do not assume this
// branch is safe by default just because it's currently unreachable in
// practice.
func defaultRoleForPlatform(platform domain.Platform) domain.Role {
	if platform == domain.PlatformCLI {
		return domain.RoleAdmin
	}
	return domain.RoleGuest
}

// ProcessMessage processes an incoming message, interacts with LLM/skills/memory, and returns an outgoing message.
// conversationID and the RBAC-relevant domain.User are both derived
// automatically from (incomingMsg.Platform, incomingMsg.PlatformUID) — the
// one-conversation-thread-per-platform-user model every gateway except the
// web Chats environment uses. See ProcessMessageInConversation for the
// alternative entry point that decouples the two.
func (s *Service) ProcessMessage(ctx context.Context, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	// 1. Validate Input
	validatedInput, err := s.securityService.ValidateInput(ctx, incomingMsg.Text, 32768) // Max 32KB for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("input validation failed: %w", err)
	}
	incomingMsg.Text = validatedInput // Use validated input

	// Generate conversation ID from platform and user
	conversationID := getConversationID(incomingMsg.Platform, incomingMsg.PlatformUID)

	// Resolve the RBAC-relevant domain.User for this platform/UID (FR-006,
	// FR-011). This must happen before confirmation-reply detection and before
	// listing/executing tools: ExecuteWithUser keys pending confirmations on
	// user.ID (the persisted domain.User's UUID, not PlatformUID), so
	// confirmation-reply detection below must look up using that same user.ID
	// or it would never find what ExecuteWithUser created
	// (specs/260803-improve-nuimanbot-security-auto-review's FR-001 fix +
	// the UserService/domain.User reconciliation — see implementation-notes.md).
	user, err := s.resolveUser(ctx, incomingMsg.Platform, incomingMsg.PlatformUID)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to resolve user: %w", err)
	}

	return s.processTurn(ctx, conversationID, user, incomingMsg)
}

// ProcessMessageInConversation processes incomingMsg within an explicit
// conversation thread (conversationID) as an already-resolved user,
// decoupling "who is asking" (RBAC identity) from "which conversation" —
// unlike ProcessMessage, neither is derived from
// (incomingMsg.Platform, incomingMsg.PlatformUID). This is what the web
// Chats environment uses: one logged-in user can own many independent named
// Chat threads (each with its own ID), which ProcessMessage's
// one-thread-per-platform-user model can't express. Callers are
// responsible for their own ownership/authorization checks before calling
// this — it performs none itself beyond RBAC on tool calls via user.
func (s *Service) ProcessMessageInConversation(ctx context.Context, conversationID string, user *domain.User, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	validatedInput, err := s.securityService.ValidateInput(ctx, incomingMsg.Text, 32768) // Max 32KB for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("input validation failed: %w", err)
	}
	incomingMsg.Text = validatedInput // Use validated input

	return s.processTurn(ctx, conversationID, user, incomingMsg)
}

// processTurn is ProcessMessage/ProcessMessageInConversation's shared core:
// everything from confirmation-reply detection through the tool-calling
// loop and turn completion, once conversationID and user are already known.
// Input validation happens in each caller (its wording — "input validation
// failed" — differs subtly in meaning depending on which entry point is
// used, so it stays there rather than here).
func (s *Service) processTurn(ctx context.Context, conversationID string, user *domain.User, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	// Add request ID to context for correlation
	ctx, reqID := requestid.MustFromContext(ctx)
	logger := requestid.Logger(ctx)

	logger.Info("Processing message",
		"platform", incomingMsg.Platform,
		"user", incomingMsg.PlatformUID,
		"conversation_id", conversationID,
	)

	// Confirmation-reply detection (Part C, FR-013): before treating
	// this message as a new chat turn, check whether it resolves an open
	// confirmation for (user.ID, conversationID).
	if s.confirmationStore != nil {
		outgoing, handled, resolveErr := s.tryResolveConfirmationReply(ctx, incomingMsg, user, conversationID, reqID, logger)
		if resolveErr != nil {
			return domain.OutgoingMessage{}, resolveErr
		}
		if handled {
			return outgoing, nil
		}
	}

	// 2. Load Conversation History
	// For MVP, retrieve recent messages for context.
	// TODO: Implement token-based trimming for context window management.
	recentMessages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, 4096) // Max 4096 tokens for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// 3. Get available skills and convert to tools, filtered by the
	// resolved user's role (FR-002 fix, FR-012).
	skills, err := s.toolExecService.ListTools(ctx, user)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to list skills: %w", err)
	}
	tools := convertSkillsToTools(skills)

	// 4. Compose system prompt with persona files (graceful degradation)
	systemPrompt := s.composeSystemPrompt(ctx, incomingMsg, logger)

	// 5. Recall long-term memories for context injection (graceful degradation)
	if s.memoryRecaller != nil {
		recalled, recallErr := s.memoryRecaller.RecallAndFormat(ctx, conversationID, incomingMsg.Text, MemoryTokenBudget)
		if recallErr != nil {
			logger.Error("Failed to recall memories",
				"conversation_id", conversationID,
				"error", recallErr,
			)
		} else if recalled != "" {
			systemPrompt = systemPrompt + "\n\n" + recalled
		}
	}

	// 6. Prepare LLM Request with tools
	llmMessages := historyToMessages(recentMessages)
	llmMessages = append(llmMessages, domain.Message{Role: "user", Content: incomingMsg.Text})

	llmRequest := &domain.LLMRequest{
		Model:        s.defaultModel(),
		Messages:     llmMessages,
		MaxTokens:    s.defaultMaxTokens(),
		Temperature:  s.defaultTemperature(),
		Tools:        tools,
		SystemPrompt: systemPrompt,
	}

	// 7. Tool calling loop (max 5 iterations). user/conversationID are
	// threaded through so every tool call in the loop is executed via
	// ExecuteWithUser (FR-001 fix) — RBAC and Part C's confirmation gate are
	// both applied per call, keyed on (user.ID, conversationID). user.ID is
	// always incomingMsg.PlatformUID (see resolveUser's doc comment), so
	// Part C's confirmation-gate keying and reply detection
	// (tryResolveConfirmationReply) are unaffected by this fix.
	//
	// enforcePublish gates runToolLoop's publish-nudge (see its doc
	// comment): ACP is the only platform where a plain LLMResponse.Content
	// is invisible to the human unless a tool call publishes it, and only
	// when that publish tool (buzz_send_message) was actually offered this
	// turn — gating on tool presence, not just platform, keeps this inert
	// for any ACP deployment that hasn't registered it.
	enforcePublish := incomingMsg.Platform == domain.PlatformACP && toolDefined(tools, buzzSendMessagePublishTool)
	finalResponse, collectedToolOutputs, pending, err := s.runToolLoop(ctx, user, conversationID, llmRequest, logger, enforcePublish)
	if err != nil {
		return domain.OutgoingMessage{}, err
	}

	// FR-013: a pending confirmation ends the current turn immediately,
	// surfacing its Summary as this turn's reply — it does not consume a
	// tool-loop iteration or count as "max iterations exceeded".
	if pending != nil {
		return s.finishPendingConfirmationTurn(ctx, incomingMsg, conversationID, reqID, pending, logger)
	}

	return s.finishTurn(ctx, incomingMsg, conversationID, reqID, finalResponse, collectedToolOutputs, logger)
}

// pendingConfirmationInfo captures a tool call's pending-confirmation
// details as surfaced by runToolLoop (Part C, FR-013).
type pendingConfirmationInfo struct {
	ID      string
	Summary string
}

// historyToMessages converts stored conversation history into the
// domain.Message shape the LLM request expects.
func historyToMessages(history []domain.StoredMessage) []domain.Message {
	messages := make([]domain.Message, 0, len(history))
	for i := range history {
		messages = append(messages, domain.Message{Role: history[i].Role, Content: history[i].Content})
	}
	return messages
}

// composeSystemPrompt builds the system prompt from persona files, falling
// back to a default prompt if no PromptComposer is configured or composition
// fails (graceful degradation — never blocks the turn).
func (s *Service) composeSystemPrompt(ctx context.Context, incomingMsg *domain.IncomingMessage, logger *slog.Logger) string {
	systemPrompt := "You are a helpful AI assistant." // Default fallback
	if s.promptComposer == nil {
		return systemPrompt
	}

	composerOutput, err := s.promptComposer.Compose(ctx, PromptComposerInput{
		UserID:   incomingMsg.PlatformUID,
		Platform: string(incomingMsg.Platform),
	})
	if err != nil {
		logger.Error("Failed to compose persona prompt",
			"user_id", incomingMsg.PlatformUID,
			"error", err,
		)
		return systemPrompt
	}

	systemPrompt = composerOutput.SystemPrompt
	if composerOutput.Truncated {
		logger.Warn("Persona prompt was truncated",
			"user_id", incomingMsg.PlatformUID,
			"tokens_used", composerOutput.TokensUsed,
			"truncated_files", composerOutput.TruncatedFiles,
		)
	}
	return systemPrompt
}

// buzzSendMessagePublishTool is internal/tools/buzzsend's registered name —
// the ACP-only tool whose invocation is the only way an ACP session's reply
// becomes a visible Buzz message (see that package's doc comment).
// runToolLoop's enforcePublish nudge (see below) checks tool calls against
// this name to decide whether a turn's reply was actually published.
const buzzSendMessagePublishTool = "buzz_send_message"

// publishNudgeText is appended as a user-role message when enforcePublish
// nudges a turn that produced content but never called
// buzzSendMessagePublishTool. Phrased as a direct, unambiguous instruction
// rather than a repeat of the general system-prompt guidance, since that
// guidance alone is what the model already had and skipped.
const publishNudgeText = "Your previous response was not published — nothing you said is visible to the human yet. " +
	"If it contains anything worth sharing, call " + buzzSendMessagePublishTool + " now, using the channel and " +
	"reply-to information from the [Context] block earlier in this conversation. If you have deliberately decided " +
	"to stay silent, respond with no tool call."

// runToolLoop runs the LLM/tool-calling loop for llmRequest, up to
// maxToolIterations rounds, starting from llmRequest.Messages (mutated in
// place as the loop progresses, same as the pre-Phase-5 inline loop this
// replaces). Every tool call is executed via ExecuteWithUser(ctx, user,
// conversationID, ...) — not the RBAC-free Execute — so RBAC and Part C's
// confirmation gate are both enforced for the resolved user (FR-001 fix, see
// resolveUser).
//
// enforcePublish, when true, makes the loop nudge the model once (see
// publishNudgeText) if a turn ends with real content but never called
// buzzSendMessagePublishTool, instead of accepting an unpublished reply as
// final. Callers should only set this for platforms where a plain
// LLMResponse.Content never reaches the human on its own (ACP/Buzz today)
// and only when that publish tool was actually offered this turn — see
// processTurn's enforcePublish computation.
//
// If any tool call in a round returns a pending-confirmation result (Part C,
// FR-013 — see pendingConfirmationFrom), the round's results are still fully
// recorded (collectedToolOutputs/llmMessages) exactly as any other round
// would be — see FR-010/FR-R10
// (specs/260803-improve-nuimanbot-security-auto-review): a pending
// confirmation on one call must gate only the loop's flow-continuation, not
// the visibility of its already-completed round-mates' real results — before
// the loop returns immediately with pending populated instead of continuing
// to iterate. Returning early this way does NOT consume one of the
// maxToolIterations rounds as a normal tool round-trip would, and must never
// be reported as "max tool calling iterations exceeded" even if detected on
// the final allowed iteration.
func (s *Service) runToolLoop(ctx context.Context, user *domain.User, conversationID string, llmRequest *domain.LLMRequest, logger *slog.Logger, enforcePublish bool) (finalResponse *domain.LLMResponse, collectedToolOutputs []string, pending *pendingConfirmationInfo, err error) {
	const baseMaxToolIterations = 5

	// maxToolIterations gets two extra rounds when enforcePublish is set, so
	// the one-shot publish nudge below never competes with real tool-call
	// rounds for the same budget. A nudge that fires costs exactly two
	// calls beyond what a normal (non-nudged) completion at that same point
	// would have used: the nudge round's own tool call, plus the follow-up
	// call every tool-call round needs afterward to process its result —
	// the same follow-up any ordinary tool round already requires, not
	// something specific to nudging. Without this, a turn that legitimately
	// needed close to baseMaxToolIterations rounds could have its nudge
	// pushed past the loop bound before finalResponse was ever set,
	// misreporting a turn that would otherwise have completed normally as
	// "max tool calling iterations exceeded" (confirmed live: the very
	// first production turn after the nudge shipped failed exactly this
	// way). The nudge itself stays bounded via `nudged` regardless of this
	// budget increase — it still fires at most once.
	maxToolIterations := baseMaxToolIterations
	if enforcePublish {
		maxToolIterations += 2
	}

	llmMessages := llmRequest.Messages

	// publishedDestinations/nudged back two related pieces of
	// enforcePublish behavior: publishedDestinations (keyed by
	// publishDestinationKey) tracks every buzz_send_message destination
	// claimed so far this turn, both to know whether the nudge below is
	// still needed (len == 0) and, via partitionPublishCalls, to skip a
	// redundant second publish to a destination already claimed this turn
	// (see that function's doc comment). nudged ensures the nudge fires at
	// most once per turn regardless of how many ordinary tool-call rounds
	// precede it.
	publishedDestinations := make(map[string]bool)
	var nudged bool

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		// Check cache before first LLM call (if cache is available)
		var llmResponse *domain.LLMResponse
		if iteration == 0 && s.cache != nil {
			cacheKey := buildCacheKey(llmMessages)
			if cached, found := s.cache.Get(ctx, cacheKey); found {
				logger.Info("Cache hit for LLM request")
				llmResponse = cached
			}
		}

		// Get LLM Response if not cached
		if llmResponse == nil {
			llmResponse, err = s.llmService.Complete(ctx, "", llmRequest) // Provider auto-resolved from model
			if err != nil {
				return nil, collectedToolOutputs, nil, fmt.Errorf("LLM completion failed: %w", err)
			}
		}

		// No tool calls. Ordinarily we'd be done -- except when
		// enforcePublish is set and this turn produced real content but
		// never called the publish tool: relying on the system prompt's
		// textual "you must publish" instruction alone has proven
		// unreliable in live ACP/Buzz testing (confirmed: the same model
		// called buzz_send_message for one turn and silently skipped it for
		// another, both carrying identical instructions), silently
		// stranding the reply where the human never sees it. Give the model
		// exactly one more forced chance before accepting silence as final
		// -- bounded by nudged so this can't loop indefinitely even if the
		// model keeps declining.
		if len(llmResponse.ToolCalls) == 0 {
			if enforcePublish && len(publishedDestinations) == 0 && !nudged && llmResponse.Content != "" {
				nudged = true
				llmMessages = append(llmMessages,
					domain.Message{Role: "assistant", Content: llmResponse.Content},
					domain.Message{Role: "user", Content: publishNudgeText},
				)
				llmRequest.Messages = llmMessages
				continue
			}

			finalResponse = llmResponse
			// Cache successful final response (no tool calls)
			if s.cache != nil && iteration == 0 {
				cacheKey := buildCacheKey(llmMessages)
				s.cache.Set(ctx, cacheKey, llmResponse)
				logger.Info("Cached LLM response")
			}
			break
		}

		// Skip any buzz_send_message call redundantly targeting a
		// destination already claimed earlier this turn (see
		// partitionPublishCalls) — claims publishedDestinations for every
		// first-time destination in this round as a side effect, which is
		// also what len(publishedDestinations) == 0 above checks to decide
		// whether the nudge is still needed.
		toolCallsToExecute, skippedResults := partitionPublishCalls(llmResponse.ToolCalls, publishedDestinations)

		// Execute tool calls and collect outputs for memory extraction. Output
		// flagged by OutputValidator (injection_flagged in Metadata) is excluded
		// from collectedToolOutputs so it can never resurface in a future
		// conversation's system prompt via the memory-curation pipeline (FR-005).
		toolResults := append(skippedResults, s.executeToolCalls(ctx, user, conversationID, toolCallsToExecute)...)

		// FR-010/FR-R10 (specs/260803-improve-nuimanbot-security-auto-review):
		// determine whether this round produced a pending confirmation, but do
		// NOT return before recording the round's results below — a pending
		// confirmation on one call must only gate this loop's
		// flow-continuation (not iterating further), never the visibility of
		// its already-completed round-mates' real results.
		pendingID, pendingSummary, hasPending := firstPendingConfirmation(toolResults)

		for _, tr := range toolResults {
			switch {
			case tr.Error != "":
				collectedToolOutputs = append(collectedToolOutputs, fmt.Sprintf("Tool %s error: %s", tr.ToolName, tr.Error))
			case isInjectionFlagged(tr.Metadata):
				logger.Warn("Excluding injection-flagged tool output from memory curation input",
					"tool", tr.ToolName,
				)
			case pendingConfirmationEntry(tr.Metadata):
				// This is the call that produced the round's pending
				// confirmation itself — it has no real Output yet (see
				// domain.StatusPendingConfirmation's doc comment), so there is
				// nothing to record for it here; its ID/Summary are surfaced
				// separately via pendingConfirmationInfo below.
			default:
				collectedToolOutputs = append(collectedToolOutputs, fmt.Sprintf("Tool %s: %s", tr.ToolName, tr.Output))
			}
		}

		// Add assistant message with tool calls to conversation
		llmMessages = append(llmMessages, domain.Message{
			Role:    "assistant",
			Content: llmResponse.Content,
		})

		// Add tool results as user message
		// Note: We need to format tool results properly for the LLM
		// For now, we'll add them as text content
		toolResultsText := formatToolResults(toolResults)
		llmMessages = append(llmMessages, domain.Message{
			Role:    "user",
			Content: toolResultsText,
		})

		// Update request with new messages
		llmRequest.Messages = llmMessages

		if hasPending {
			return nil, collectedToolOutputs, &pendingConfirmationInfo{ID: pendingID, Summary: pendingSummary}, nil
		}
	}

	// If we hit max iterations, use last response
	if finalResponse == nil {
		return nil, collectedToolOutputs, nil, fmt.Errorf("max tool calling iterations exceeded")
	}

	return finalResponse, collectedToolOutputs, nil, nil
}

// finishTurn completes a normal (non-pending) turn: memory-cell extraction,
// persisting the incoming/outgoing messages, and building the final
// OutgoingMessage.
func (s *Service) finishTurn(ctx context.Context, incomingMsg *domain.IncomingMessage, conversationID, reqID string, finalResponse *domain.LLMResponse, collectedToolOutputs []string, logger *slog.Logger) (domain.OutgoingMessage, error) {
	responseContent := finalResponse.Content

	// Extract memory cells from the interaction (non-blocking, graceful degradation)
	if s.memoryCurator != nil {
		if curatorErr := s.memoryCurator.ExtractMemoryCells(ctx, conversationID, incomingMsg.Text, responseContent, collectedToolOutputs); curatorErr != nil {
			logger.Error("Failed to extract memory cells",
				"conversation_id", conversationID,
				"error", curatorErr,
			)
		}
	}

	s.saveTurnMessages(ctx, conversationID, incomingMsg, responseContent, finalResponse.Usage.CompletionTokens, logger)

	return domain.OutgoingMessage{
		RecipientID: incomingMsg.PlatformUID, // Send back to the same user
		Content:     responseContent,
		Format:      "markdown",                          // Assuming LLM returns markdown
		Metadata:    map[string]any{"request_id": reqID}, // Include request ID for correlation
	}, nil
}

// finishPendingConfirmationTurn completes a turn that ended on a pending
// confirmation (Part C, FR-013): persists the turn's messages (using the
// confirmation Summary as the assistant's reply) and returns it as the
// OutgoingMessage, tagged with Metadata gateway agents (P5.7-P5.9) can use to
// render an interactive yes/no prompt.
func (s *Service) finishPendingConfirmationTurn(ctx context.Context, incomingMsg *domain.IncomingMessage, conversationID, reqID string, pending *pendingConfirmationInfo, logger *slog.Logger) (domain.OutgoingMessage, error) {
	s.saveTurnMessages(ctx, conversationID, incomingMsg, pending.Summary, 0, logger)

	return domain.OutgoingMessage{
		RecipientID: incomingMsg.PlatformUID,
		Content:     pending.Summary,
		Format:      "markdown",
		Metadata: map[string]any{
			"request_id":      reqID,
			"status":          "pending_confirmation",
			"confirmation_id": pending.ID,
		},
	}, nil
}

// ResolveConfirmation resolves the confirmation identified by confirmationID
// — approving or denying it — and, on approval, re-invokes the originally
// requested tool call and runs a fresh tool-loop turn, exactly like the
// chat-gateway yes/no reply path (resolveConfirmationApproved/Denied). It is
// the entry point non-gateway callers use to resolve a confirmation that
// isn't tied to a live incoming chat message — e.g. the REST API's
// POST /api/v1/confirmations/{id}/resolve handler (Part C, FR-011, P5.9).
//
// Returns an error if no ConfirmationStore is configured for this Service
// (SetConfirmationStore was never called), or if confirmationID does not
// identify a resolvable confirmation. security.ErrConfirmationNotFound and
// security.ErrConfirmationAlreadyResolved propagate unwrapped (checkable via
// errors.Is) so callers can distinguish "no such confirmation" from "already
// resolved" — e.g. to map them to distinct HTTP status codes.
func (s *Service) ResolveConfirmation(ctx context.Context, confirmationID string, approved bool) (domain.OutgoingMessage, error) {
	if s.confirmationStore == nil {
		return domain.OutgoingMessage{}, fmt.Errorf("resolve confirmation: no confirmation store configured")
	}

	ctx, reqID := requestid.MustFromContext(ctx)
	logger := requestid.Logger(ctx)

	req, err := s.confirmationStore.Get(ctx, confirmationID)
	if err != nil {
		return domain.OutgoingMessage{}, err
	}

	// Synthesize an IncomingMessage carrying just enough identity for
	// saveTurnMessages/resolveConfirmationApproved/resolveConfirmationDenied
	// to record the resulting turn against the right conversation —
	// PlatformUID/ConversationID come from the stored confirmation, not a
	// live platform message, since this call has no such message.
	incomingMsg := &domain.IncomingMessage{
		ID:          "confirmation-resolve-" + confirmationID,
		Platform:    domain.Platform("api"),
		PlatformUID: req.UserID,
		Text:        confirmationResolutionText(approved),
		Timestamp:   time.Now(),
	}

	if approved {
		return s.resolveConfirmationApproved(ctx, incomingMsg, req.ConversationID, reqID, req, logger)
	}
	return s.resolveConfirmationDenied(ctx, incomingMsg, req.ConversationID, reqID, req, logger)
}

// confirmationResolutionText returns the plain-text yes/no vocabulary word
// recorded as the synthetic user turn for a non-gateway-initiated
// ResolveConfirmation call, so the conversation history reads naturally.
func confirmationResolutionText(approved bool) string {
	if approved {
		return "yes"
	}
	return "no"
}

// saveTurnMessages persists the incoming user message and an outgoing
// assistant message (with the given reply content and token count) to
// memory. Best-effort: failures are logged, not returned, matching the
// pre-Phase-5 behavior this was extracted from.
func (s *Service) saveTurnMessages(ctx context.Context, conversationID string, incomingMsg *domain.IncomingMessage, replyContent string, tokenCount int, logger *slog.Logger) {
	incomingStoredMsg := domain.StoredMessage{
		ID:        incomingMsg.ID, // Use incoming message ID
		Role:      "user",
		Content:   incomingMsg.Text,
		Timestamp: incomingMsg.Timestamp,
	}
	if err := s.memoryRepo.SaveMessage(ctx, conversationID, incomingMsg.PlatformUID, incomingMsg.Platform, incomingStoredMsg); err != nil {
		logger.Error("Error saving incoming message to memory",
			"conversation_id", conversationID,
			"error", err,
		)
	}

	outgoingStoredMsg := domain.StoredMessage{
		ID:         "bot-response-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Role:       "assistant",
		Content:    replyContent,
		Timestamp:  time.Now(),
		TokenCount: tokenCount,
	}
	if err := s.memoryRepo.SaveMessage(ctx, conversationID, incomingMsg.PlatformUID, incomingMsg.Platform, outgoingStoredMsg); err != nil {
		logger.Error("Error saving outgoing message to memory",
			"conversation_id", conversationID,
			"error", err,
		)
	}
}

// confirmationReplyResolution classifies whether an incoming message
// resolves an open confirmation (Part C, FR-013's yes/no heuristic).
type confirmationReplyResolution int

const (
	// confirmationReplyAmbiguous means the message neither approves nor
	// denies the open confirmation — it is left pending and the message is
	// processed as a normal new turn (FR-013).
	confirmationReplyAmbiguous confirmationReplyResolution = iota
	confirmationReplyApprove
	confirmationReplyDeny
)

// confirmationApproveWords/confirmationDenyWords are the plain-text yes/no
// vocabulary (specs/260802-improve-nuimanbot-security §5.3.3's "universal
// fallback", case-insensitive, exact match after trimming whitespace).
var (
	confirmationApproveWords = map[string]bool{"yes": true, "y": true, "approve": true, "confirm": true}
	confirmationDenyWords    = map[string]bool{"no": true, "n": true, "deny": true, "cancel": true, "reject": true}
)

// classifyConfirmationReply implements Part C's yes/no heuristic (FR-013).
func classifyConfirmationReply(text string) confirmationReplyResolution {
	normalized := strings.ToLower(strings.TrimSpace(text))
	switch {
	case confirmationApproveWords[normalized]:
		return confirmationReplyApprove
	case confirmationDenyWords[normalized]:
		return confirmationReplyDeny
	default:
		return confirmationReplyAmbiguous
	}
}

// tryResolveConfirmationReply checks whether incomingMsg resolves an open
// confirmation for (user.ID, conversationID) (Part C, FR-013). Looked up by
// the resolved domain.User's ID — not incomingMsg.PlatformUID — because
// ExecuteWithUser creates pending confirmations keyed on user.ID, which is a
// persisted UUID (UserService.CreateUser), not the raw platform identifier;
// using PlatformUID here would never match what was actually stored. handled
// is true when it does (approve or deny) — the caller must return
// outgoing/err directly, without any further normal-turn processing. handled
// is false when there is no open confirmation, or when one exists but this
// message doesn't resolve it (left pending; the message must still be
// processed as a normal new turn per FR-013).
func (s *Service) tryResolveConfirmationReply(ctx context.Context, incomingMsg *domain.IncomingMessage, user *domain.User, conversationID, reqID string, logger *slog.Logger) (domain.OutgoingMessage, bool, error) {
	req, open, err := s.confirmationStore.GetOpenByKey(ctx, user.ID, conversationID)
	if err != nil {
		// A lookup failure isn't an execution-safety concern (nothing is
		// about to run unconfirmed) — fail open to normal turn processing
		// rather than blocking the user's message entirely.
		logger.Error("Failed to look up open confirmation", "conversation_id", conversationID, "error", err)
		return domain.OutgoingMessage{}, false, nil
	}
	if !open {
		return domain.OutgoingMessage{}, false, nil
	}

	switch classifyConfirmationReply(incomingMsg.Text) {
	case confirmationReplyApprove:
		outgoing, err := s.resolveConfirmationApproved(ctx, incomingMsg, conversationID, reqID, req, logger)
		return outgoing, true, err
	case confirmationReplyDeny:
		outgoing, err := s.resolveConfirmationDenied(ctx, incomingMsg, conversationID, reqID, req, logger)
		return outgoing, true, err
	default:
		logger.Info("Message did not resolve the open confirmation; processing as a new turn",
			"confirmation_id", req.ID,
		)
		return domain.OutgoingMessage{}, false, nil
	}
}

// resolveConfirmationDenied resolves req as denied and reports the
// cancellation to the user. The originally-requested tool call is never
// executed.
func (s *Service) resolveConfirmationDenied(ctx context.Context, incomingMsg *domain.IncomingMessage, conversationID, reqID string, req security.ConfirmationRequest, logger *slog.Logger) (domain.OutgoingMessage, error) {
	if _, err := s.confirmationStore.Resolve(ctx, req.ID, false); err != nil {
		// The user's intent to deny is unambiguous regardless of whether the
		// store's bookkeeping succeeded (and denial never executes anything,
		// so there's no fail-closed *execution* concern here) — log and
		// still report the cancellation.
		logger.Error("Failed to resolve confirmation as denied", "confirmation_id", req.ID, "error", err)
	}

	content := fmt.Sprintf("Cancelled: %s", req.Summary)
	s.saveTurnMessages(ctx, conversationID, incomingMsg, content, 0, logger)

	return domain.OutgoingMessage{
		RecipientID: incomingMsg.PlatformUID,
		Content:     content,
		Format:      "markdown",
		Metadata: map[string]any{
			"request_id":      reqID,
			"status":          "confirmation_denied",
			"confirmation_id": req.ID,
		},
	}, nil
}

// resolveConfirmationApproved resolves req as approved, re-invokes the
// originally-requested tool call directly with its original parameters
// (bypassing a fresh LLM prompt/re-decision, per FR-013), and feeds that
// tool's result into a NEW, fresh tool-loop invocation with its own 5-
// iteration budget so the model can react to the now-completed action.
func (s *Service) resolveConfirmationApproved(ctx context.Context, incomingMsg *domain.IncomingMessage, conversationID, reqID string, req security.ConfirmationRequest, logger *slog.Logger) (domain.OutgoingMessage, error) {
	resolved, err := s.confirmationStore.Resolve(ctx, req.ID, true)
	if err != nil {
		logger.Error("Failed to resolve confirmation as approved", "confirmation_id", req.ID, "error", err)
		return domain.OutgoingMessage{}, fmt.Errorf("failed to resolve confirmation: %w", err)
	}

	// Re-invoke directly via the RBAC/confirmation-free Execute — this call
	// has already been approved and must not be re-gated. Deliberately does
	// NOT attach a security.ConfirmationIdentity to ctx here.
	result, execErr := s.toolExecService.Execute(ctx, resolved.ToolName, resolved.Params)
	toolResult := domain.ToolResult{ToolName: resolved.ToolName}
	switch {
	case execErr != nil:
		toolResult.Error = execErr.Error()
	case result.Error != "":
		toolResult.Error = result.Error
	default:
		toolResult.Output = result.Output
		toolResult.Metadata = result.Metadata
	}

	recentMessages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, 4096)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// Resolve a role-bearing identity for RBAC (FR-001/FR-002 fix) — the
	// fresh tool-loop invocation below is a normal turn like any other, so
	// any further tool calls the model makes in reaction to the
	// now-completed approved action must go through ExecuteWithUser, not
	// Execute.
	user, err := s.resolveUser(ctx, incomingMsg.Platform, incomingMsg.PlatformUID)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to resolve user: %w", err)
	}

	skills, err := s.toolExecService.ListTools(ctx, user)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to list skills: %w", err)
	}
	tools := convertSkillsToTools(skills)

	systemPrompt := s.composeSystemPrompt(ctx, incomingMsg, logger)

	llmMessages := historyToMessages(recentMessages)
	llmMessages = append(llmMessages, domain.Message{Role: "user", Content: formatToolResults([]domain.ToolResult{toolResult})})

	llmRequest := &domain.LLMRequest{
		Model:        s.defaultModel(),
		Messages:     llmMessages,
		MaxTokens:    s.defaultMaxTokens(),
		Temperature:  s.defaultTemperature(),
		Tools:        tools,
		SystemPrompt: systemPrompt,
	}

	enforcePublish := incomingMsg.Platform == domain.PlatformACP && toolDefined(tools, buzzSendMessagePublishTool)
	finalResponse, collectedToolOutputs, pending, err := s.runToolLoop(ctx, user, conversationID, llmRequest, logger, enforcePublish)
	if err != nil {
		return domain.OutgoingMessage{}, err
	}
	if pending != nil {
		// Rare but possible: the fresh tool-loop invocation itself triggered
		// another confirmation-required action. Handle it exactly like any
		// other pending confirmation rather than erroring.
		return s.finishPendingConfirmationTurn(ctx, incomingMsg, conversationID, reqID, pending, logger)
	}

	return s.finishTurn(ctx, incomingMsg, conversationID, reqID, finalResponse, collectedToolOutputs, logger)
}
