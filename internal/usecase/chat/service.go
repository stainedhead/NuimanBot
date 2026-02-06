package chat

import (
	"context"
	"fmt"
	"log"  // For logging errors in defer functions etc.
	"time" // For time.Now()

	"nuimanbot/internal/domain"
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

// SkillExecutionService defines the interface for skill execution required by the ChatService.
// This will be defined by the Skills Core Agent.
type SkillExecutionService interface {
	Execute(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error)
	ListSkills(ctx context.Context, userID string) ([]domain.Skill, error)
	// Other methods for skill management (e.g., registration, permission checks)
}

// SecurityService defines the interface for security operations required by the ChatService.
// This is effectively a subset or exact copy of domain.SecurityService.
type SecurityService interface {
	ValidateInput(ctx context.Context, input string, maxLength int) (string, error)
	Audit(ctx context.Context, event *domain.AuditEvent) error
	// Other methods from SecurityService, e.g., Encrypt/Decrypt if chat needs them
}

// Service implements the ChatService use case.
type Service struct {
	llmService       LLMService
	memoryRepo       MemoryRepository
	skillExecService SkillExecutionService // Currently PENDING (will be mocked or basic for now)
	securityService  SecurityService
	// config            *config.ChatConfig // If ChatService needs its own config
}

// NewService creates a new ChatService instance.
func NewService(
	llmService LLMService,
	memoryRepo MemoryRepository,
	skillExecService SkillExecutionService,
	securityService SecurityService,
) *Service {
	return &Service{
		llmService:       llmService,
		memoryRepo:       memoryRepo,
		skillExecService: skillExecService,
		securityService:  securityService,
	}
}

// getConversationID generates a conversation ID based on platform and user
func getConversationID(platform domain.Platform, platformUID string) string {
	return string(platform) + ":" + platformUID
}

// ProcessMessage processes an incoming message, interacts with LLM/skills/memory, and returns an outgoing message.
func (s *Service) ProcessMessage(ctx context.Context, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	// 1. Validate Input
	validatedInput, err := s.securityService.ValidateInput(ctx, incomingMsg.Text, 32768) // Max 32KB for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("input validation failed: %w", err)
	}
	incomingMsg.Text = validatedInput // Use validated input

	// Generate conversation ID from platform and user
	conversationID := getConversationID(incomingMsg.Platform, incomingMsg.PlatformUID)

	// 2. Load Conversation History
	// For MVP, retrieve recent messages for context.
	// TODO: Implement token-based trimming for context window management.
	recentMessages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, 4096) // Max 4096 tokens for now
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to get recent messages: %w", err)
	}

	// 3. Get available skills and convert to tools
	// Note: Using PlatformUID as user identifier for skill permissions
	skills, err := s.skillExecService.ListSkills(ctx, incomingMsg.PlatformUID)
	if err != nil {
		return domain.OutgoingMessage{}, fmt.Errorf("failed to list skills: %w", err)
	}
	tools := convertSkillsToTools(skills)

	// 4. Prepare LLM Request with tools
	llmMessages := []domain.Message{}
	// Add system prompt if any (TODO: from config)
	// Add history
	for i := range recentMessages {
		llmMessages = append(llmMessages, domain.Message{Role: recentMessages[i].Role, Content: recentMessages[i].Content})
	}
	// Add current message
	llmMessages = append(llmMessages, domain.Message{Role: "user", Content: incomingMsg.Text})

	llmRequest := &domain.LLMRequest{
		Model:        "claude-3-sonnet-20240229", // TODO: Get from config/user preferences
		Messages:     llmMessages,
		MaxTokens:    1024,                              // TODO: From config
		Temperature:  0.7,                               // TODO: From config
		Tools:        tools,                             // Skills exposed as tools
		SystemPrompt: "You are a helpful AI assistant.", // TODO: From config
	}

	// 5. Tool calling loop (max 5 iterations)
	const maxToolIterations = 5
	var finalResponse *domain.LLMResponse

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		// Get LLM Response
		llmResponse, err := s.llmService.Complete(ctx, domain.LLMProviderAnthropic, llmRequest) // TODO: Route dynamically
		if err != nil {
			return domain.OutgoingMessage{}, fmt.Errorf("LLM completion failed: %w", err)
		}

		// No tool calls - we're done
		if len(llmResponse.ToolCalls) == 0 {
			finalResponse = llmResponse
			break
		}

		// Execute tool calls
		toolResults := s.executeToolCalls(ctx, llmResponse.ToolCalls)

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

	// 6. Process final LLM Response
	responseContent := finalResponse.Content

	// 6. Save new messages to memory (incoming and outgoing)
	incomingStoredMsg := domain.StoredMessage{
		ID:        incomingMsg.ID, // Use incoming message ID
		Role:      "user",
		Content:   incomingMsg.Text,
		Timestamp: incomingMsg.Timestamp,
		// TokenCount:  llmRequest.Tokens(), // TODO: Calculate actual token count
	}
	if err := s.memoryRepo.SaveMessage(ctx, conversationID, incomingMsg.PlatformUID, incomingMsg.Platform, incomingStoredMsg); err != nil {
		log.Printf("Error saving incoming message to memory: %v", err)
	}

	outgoingStoredMsg := domain.StoredMessage{
		ID:         "bot-response-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Role:       "assistant",
		Content:    responseContent,
		Timestamp:  time.Now(),
		TokenCount: finalResponse.Usage.CompletionTokens, // Using LLM's reported tokens
	}
	if err := s.memoryRepo.SaveMessage(ctx, conversationID, incomingMsg.PlatformUID, incomingMsg.Platform, outgoingStoredMsg); err != nil {
		log.Printf("Error saving outgoing message to memory: %v", err)
	}

	// 7. Return Outgoing Message
	outgoingMsg := domain.OutgoingMessage{
		RecipientID: incomingMsg.PlatformUID, // Send back to the same user
		Content:     responseContent,
		Format:      "markdown", // Assuming LLM returns markdown
		Metadata:    nil,
	}

	return outgoingMsg, nil
}
