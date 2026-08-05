// Package memories orchestrates the web admin's Memories environment
// (FR-045–FR-047): a read-only browse/search view over the agent-maintained
// memoryv2 memory-cell store, plus a minimal per-cell chat (FR-047/FR-R4's
// AskAboutCell). There is no create/update/delete of memory cells — the
// agent (via the real curator path) is the store's sole writer of cell
// content.
//
// ownerUserID -> ConversationID mapping (CONFIRMED GAP, FR-R7 — no longer an
// unverified assumption; see
// TestFR_R7_ConversationIDMapping_TracesRealCLIGatewayFormat in
// conversation_id_mapping_test.go for the traced evidence): tracing the real
// CLI gateway end-to-end (internal/adapter/gateway/cli/gateway.go ->
// internal/usecase/chat.Service.getConversationID(platform, platformUID) ->
// internal/usecase/memoryv2.MemoryCuratorService.ExtractCells) proves memory
// cells are actually keyed by "<platform>:<platformUID>" (e.g.
// "cli:cli_user" — every CLI interaction today, since the CLI gateway
// hardcodes a single placeholder PlatformUID for all users), not by the
// web-admin session Username this package receives as ownerUserID
// everywhere else. Confirmed further: internal/adapter/web's login/session
// handling never references domain.UserRepository/domain.User at all, so
// there is no existing bridge in this codebase from a web-admin account to
// a domain.User/platform identity to map through even if this Service tried
// to look one up. Building that bridge (and fixing the CLI gateway's own
// single-shared-placeholder identity) is a separately-scoped effort, out of
// this fix pass for the same reason FR-R1's live Chats reply loop is —- see
// implementation-notes.md's Deviations from Plan. Pending that, this
// Service keeps the explicit, documented (not silent) choice of treating
// ownerUserID itself as the ConversationID filter value (conversationIDFor
// below); the Memories UI carries a visible notice that it may not show all
// of a user's stored knowledge as a result.
package memories

import (
	"context"
	"fmt"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
)

// LLMService is the minimal single-turn completion interface Service needs
// for AskAboutCell (FR-R4's per-item chat template — Memory is the reference
// implementation the other three items, Job/Chore/Run, are meant to follow
// in a fast-follow pass). Deliberately NOT internal/usecase/chat.Service's
// full multi-turn/tool-calling/RBAC orchestration engine — that integration
// is FR-R1's live Chats reply loop, explicitly out of scope for this pass
// per the PRD's own Non-Goals. This is one grounded LLM call per question,
// with no conversation history, no tools, and no RBAC surface of its own
// (ownership is already enforced by GetCell before this interface is ever
// reached).
type LLMService interface {
	Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error)
}

// LLMDefaults mirrors internal/usecase/chat.LLMDefaults's shape (Model/
// MaxTokens/Temperature) so the same config.LLM.DefaultModel values used to
// configure the main Chats orchestration can also configure this narrower,
// single-turn per-cell chat.
type LLMDefaults struct {
	Model       string
	MaxTokens   int
	Temperature float64
}

// Service provides read-only, per-user-scoped access to the memoryv2
// memory-cell store (FR-045). It never calls Create/Update/Delete on the
// underlying repository (FR-046). AskAboutCell is the sole exception to
// "read-only" in the sense that it makes an outbound LLM call, but it never
// mutates the memory-cell store itself.
type Service struct {
	cells       memoryv2.MemoryCellRepository
	llm         LLMService
	llmDefaults LLMDefaults
}

// Option configures optional Service dependencies at construction time.
type Option func(*Service)

// WithLLM wires the LLMService AskAboutCell uses. Omit it (e.g. in a
// deployment/test that never calls AskAboutCell) and AskAboutCell returns a
// clear error instead of panicking.
func WithLLM(llm LLMService) Option {
	return func(s *Service) { s.llm = llm }
}

// WithLLMDefaults configures the Model/MaxTokens/Temperature AskAboutCell
// uses, mirroring chat.Service.SetLLMDefaults's config source
// (cfg.LLM.DefaultModel).
func WithLLMDefaults(defaults LLMDefaults) Option {
	return func(s *Service) { s.llmDefaults = defaults }
}

// NewService creates a memories Service backed by cells. Pass WithLLM (and
// optionally WithLLMDefaults) to enable AskAboutCell; without it, browsing
// (ListCells/GetCell) still works but AskAboutCell returns an error.
func NewService(cells memoryv2.MemoryCellRepository, opts ...Option) *Service {
	s := &Service{cells: cells}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// defaultModel returns the configured default model or a sensible fallback,
// matching chat.Service.defaultModel's convention.
func (s *Service) defaultModel() string {
	if s.llmDefaults.Model != "" {
		return s.llmDefaults.Model
	}
	return "claude-3-sonnet-20240229"
}

func (s *Service) defaultMaxTokens() int {
	if s.llmDefaults.MaxTokens > 0 {
		return s.llmDefaults.MaxTokens
	}
	return 1024
}

func (s *Service) defaultTemperature() float64 {
	if s.llmDefaults.Temperature > 0 {
		return s.llmDefaults.Temperature
	}
	return 0.7
}

// conversationIDFor maps ownerUserID to the ConversationID scoping value
// used to filter memoryv2 cells. See the package doc comment for the
// assumption this encodes.
func conversationIDFor(ownerUserID string) string {
	return ownerUserID
}

// ListCells returns memory cells visible to ownerUserID matching filter
// (FR-045's browse/search view). filter.ConversationID is always overridden
// from ownerUserID's mapping, regardless of what the caller passed — this
// is the isolation enforcement point that prevents a caller from viewing
// another user's cells by manipulating the filter.
func (s *Service) ListCells(ctx context.Context, ownerUserID string, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	filter.ConversationID = conversationIDFor(ownerUserID)
	return s.cells.List(ctx, filter)
}

// GetCell retrieves a single memory cell by ID, scoped to ownerUserID's
// visibility. A cell that doesn't exist resolves as memoryv2.ErrNotFound (the
// repository's own not-found error); a cell that exists but belongs to a
// different owner resolves as domain.ErrNotFound (FR-010/Edge Case #10 —
// existence is never disclosed across owners).
func (s *Service) GetCell(ctx context.Context, ownerUserID, cellID string) (*memoryv2.MemoryCell, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	cell, err := s.cells.Get(ctx, cellID)
	if err != nil {
		return nil, err
	}
	if cell.ConversationID != conversationIDFor(ownerUserID) {
		return nil, domain.ErrNotFound
	}
	return cell, nil
}

// AskAboutCell answers a single-turn question grounded only in one memory
// cell's own content (FR-047/FR-R4 — Memory is the reference implementation
// of the per-item chat template FR-R4 requires; Job/Chore/Run follow the
// same shape in a fast-follow pass). Ownership is enforced by delegating to
// GetCell first, so a cell belonging to a different owner resolves as
// domain.ErrNotFound before any LLM call is made, and existence is never
// disclosed across owners (same as every other read path in this Service).
func (s *Service) AskAboutCell(ctx context.Context, ownerUserID, cellID, question string) (string, error) {
	if question == "" {
		return "", fmt.Errorf("%w: question is required", domain.ErrInvalidInput)
	}
	if s.llm == nil {
		return "", fmt.Errorf("%w: chat is not configured for Memories", domain.ErrInvalidInput)
	}
	cell, err := s.GetCell(ctx, ownerUserID, cellID)
	if err != nil {
		return "", err
	}

	req := &domain.LLMRequest{
		Model:        s.defaultModel(),
		MaxTokens:    s.defaultMaxTokens(),
		Temperature:  s.defaultTemperature(),
		SystemPrompt: groundingPromptFor(cell),
		Messages:     []domain.Message{{Role: "user", Content: question}},
	}
	resp, err := s.llm.Complete(ctx, "", req) // provider auto-resolved from model, matching chat.Service's convention
	if err != nil {
		return "", fmt.Errorf("failed to get an answer: %w", err)
	}
	return resp.Content, nil
}

// groundingPromptFor builds the system prompt that confines the LLM's
// answer to a single memory cell's own content — the "grounded in that
// item's own context" requirement from FR-R4's acceptance criteria.
func groundingPromptFor(cell *memoryv2.MemoryCell) string {
	return fmt.Sprintf(
		"You are answering a question about a single stored memory entry. "+
			"Base your answer only on the memory content below. If the question "+
			"cannot be answered from it, say so plainly rather than guessing.\n\n"+
			"Scene: %s\nType: %s\nSalience: %.2f\nCreated: %s\nContent: %s",
		cell.Scene,
		cell.CellType.String(),
		cell.Salience,
		cell.CreatedAt.Format(time.RFC3339),
		cell.Content,
	)
}
