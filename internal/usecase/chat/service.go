package chat

import (
	"context"
	"errors"
	"fmt"
	"time" // For time.Now()

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/requestid"
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
	ExecuteWithUser(ctx context.Context, user *domain.User, toolName string, params map[string]any) (*domain.ExecutionResult, error)
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
func defaultRoleForPlatform(platform domain.Platform) domain.Role {
	if platform == domain.PlatformCLI {
		return domain.RoleAdmin
	}
	return domain.RoleGuest
}

// ProcessMessage processes an incoming message, interacts with LLM/skills/memory, and returns an outgoing message.
func (s *Service) ProcessMessage(ctx context.Context, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	// Add request ID to context for correlation
	ctx, reqID := requestid.MustFromContext(ctx)
	logger := requestid.Logger(ctx)

	logger.Info("Processing message",
		"platform", incomingMsg.Platform,
		"user", incomingMsg.PlatformUID,
	)

	// 1. Validate Input
	validatedInput, err := s.securityService.ValidateInput(ctx, incomingMsg.Text, 32768) // Max 32KB for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("input validation failed: %w", err)
	}
	incomingMsg.Text = validatedInput // Use validated input

	// Generate conversation ID from platform and user
	conversationID := getConversationID(incomingMsg.Platform, incomingMsg.PlatformUID)

	// 1.5. Resolve the RBAC-relevant domain.User for this platform/UID (FR-006,
	// FR-011). This must happen before listing/executing tools so those calls
	// are genuinely permission-checked, not just labeled with a raw UID.
	user, err := s.resolveUser(ctx, incomingMsg.Platform, incomingMsg.PlatformUID)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to resolve user: %w", err)
	}

	// 2. Load Conversation History
	// For MVP, retrieve recent messages for context.
	// TODO: Implement token-based trimming for context window management.
	recentMessages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, 4096) // Max 4096 tokens for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// 3. Get available skills and convert to tools (role-filtered, FR-012)
	skills, err := s.toolExecService.ListTools(ctx, user)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to list skills: %w", err)
	}
	tools := convertSkillsToTools(skills)

	// 4. Compose system prompt with persona files (graceful degradation)
	systemPrompt := "You are a helpful AI assistant." // Default fallback
	if s.promptComposer != nil {
		composerOutput, composeErr := s.promptComposer.Compose(ctx, PromptComposerInput{
			UserID:   incomingMsg.PlatformUID,
			Platform: string(incomingMsg.Platform),
		})
		if composeErr != nil {
			logger.Error("Failed to compose persona prompt",
				"user_id", incomingMsg.PlatformUID,
				"error", composeErr,
			)
			// Fallback to default prompt
		} else {
			systemPrompt = composerOutput.SystemPrompt
			if composerOutput.Truncated {
				logger.Warn("Persona prompt was truncated",
					"user_id", incomingMsg.PlatformUID,
					"tokens_used", composerOutput.TokensUsed,
					"truncated_files", composerOutput.TruncatedFiles,
				)
			}
		}
	}

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
	llmMessages := []domain.Message{}
	// Add history
	for i := range recentMessages {
		llmMessages = append(llmMessages, domain.Message{Role: recentMessages[i].Role, Content: recentMessages[i].Content})
	}
	// Add current message
	llmMessages = append(llmMessages, domain.Message{Role: "user", Content: incomingMsg.Text})

	llmRequest := &domain.LLMRequest{
		Model:        s.defaultModel(),
		Messages:     llmMessages,
		MaxTokens:    s.defaultMaxTokens(),
		Temperature:  s.defaultTemperature(),
		Tools:        tools,
		SystemPrompt: systemPrompt,
	}

	// 7. Tool calling loop (max 5 iterations)
	const maxToolIterations = 5
	var finalResponse *domain.LLMResponse
	var collectedToolOutputs []string

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
			var err error
			llmResponse, err = s.llmService.Complete(ctx, "", llmRequest) // Provider auto-resolved from model
			if err != nil {
				return domain.OutgoingMessage{}, fmt.Errorf("LLM completion failed: %w", err)
			}
		}

		// No tool calls - we're done
		if len(llmResponse.ToolCalls) == 0 {
			finalResponse = llmResponse
			// Cache successful final response (no tool calls)
			if s.cache != nil && iteration == 0 {
				cacheKey := buildCacheKey(llmMessages)
				s.cache.Set(ctx, cacheKey, llmResponse)
				logger.Info("Cached LLM response")
			}
			break
		}

		// Execute tool calls and collect outputs for memory extraction
		toolResults := s.executeToolCalls(ctx, user, llmResponse.ToolCalls)
		for _, tr := range toolResults {
			if tr.Error != "" {
				collectedToolOutputs = append(collectedToolOutputs, fmt.Sprintf("Tool %s error: %s", tr.ToolName, tr.Error))
			} else {
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
	}

	// If we hit max iterations, use last response
	if finalResponse == nil {
		return domain.OutgoingMessage{}, fmt.Errorf("max tool calling iterations exceeded")
	}

	// 7. Process final LLM Response
	responseContent := finalResponse.Content

	// 8. Extract memory cells from the interaction (non-blocking, graceful degradation)
	if s.memoryCurator != nil {
		if curatorErr := s.memoryCurator.ExtractMemoryCells(ctx, conversationID, incomingMsg.Text, responseContent, collectedToolOutputs); curatorErr != nil {
			logger.Error("Failed to extract memory cells",
				"conversation_id", conversationID,
				"error", curatorErr,
			)
		}
	}

	// 9. Save new messages to memory (incoming and outgoing)
	incomingStoredMsg := domain.StoredMessage{
		ID:        incomingMsg.ID, // Use incoming message ID
		Role:      "user",
		Content:   incomingMsg.Text,
		Timestamp: incomingMsg.Timestamp,
		// TokenCount:  llmRequest.Tokens(), // TODO: Calculate actual token count
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
		Content:    responseContent,
		Timestamp:  time.Now(),
		TokenCount: finalResponse.Usage.CompletionTokens, // Using LLM's reported tokens
	}
	if err := s.memoryRepo.SaveMessage(ctx, conversationID, incomingMsg.PlatformUID, incomingMsg.Platform, outgoingStoredMsg); err != nil {
		logger.Error("Error saving outgoing message to memory",
			"conversation_id", conversationID,
			"error", err,
		)
	}

	// 10. Return Outgoing Message
	outgoingMsg := domain.OutgoingMessage{
		RecipientID: incomingMsg.PlatformUID, // Send back to the same user
		Content:     responseContent,
		Format:      "markdown",                          // Assuming LLM returns markdown
		Metadata:    map[string]any{"request_id": reqID}, // Include request ID for correlation
	}

	return outgoingMsg, nil
}
