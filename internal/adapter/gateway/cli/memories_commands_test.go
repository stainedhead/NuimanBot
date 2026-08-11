package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/usecase/memories"
)

// fakeCellRepo is a minimal in-memory memoryv2.MemoryCellRepository test
// double for this package. It can't reuse internal/usecase/memories's own
// unexported fakeCellRepo (different package), but mirrors its List/Get
// semantics closely enough for MemoriesCommandHandler's tests.
type fakeCellRepo struct {
	cells map[string]*memoryv2.MemoryCell
}

func newFakeCellRepo() *fakeCellRepo {
	return &fakeCellRepo{cells: make(map[string]*memoryv2.MemoryCell)}
}

func (f *fakeCellRepo) Create(_ context.Context, cell *memoryv2.MemoryCell) error {
	f.cells[cell.ID] = cell
	return nil
}

func (f *fakeCellRepo) Get(_ context.Context, id string) (*memoryv2.MemoryCell, error) {
	cell, ok := f.cells[id]
	if !ok {
		return nil, memoryv2.ErrNotFound
	}
	return cell, nil
}

func (f *fakeCellRepo) List(_ context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	out := []*memoryv2.MemoryCell{}
	for _, cell := range f.cells {
		if filter.ConversationID != "" && cell.ConversationID != filter.ConversationID {
			continue
		}
		if filter.Scene != "" && cell.Scene != filter.Scene {
			continue
		}
		out = append(out, cell)
	}
	return out, nil
}

func (f *fakeCellRepo) Update(_ context.Context, _ *memoryv2.MemoryCell) error {
	return errors.New("not implemented: unused by MemoriesCommandHandler tests")
}

func (f *fakeCellRepo) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented: unused by MemoriesCommandHandler tests")
}

func (f *fakeCellRepo) SearchFTS(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, errors.New("not implemented: unused by MemoriesCommandHandler tests")
}

func (f *fakeCellRepo) GetByScene(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, errors.New("not implemented: unused by MemoriesCommandHandler tests")
}

func (f *fakeCellRepo) GetHighSalience(_ context.Context, _ string, _ float64, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, errors.New("not implemented: unused by MemoriesCommandHandler tests")
}

func (f *fakeCellRepo) DeleteExpired(_ context.Context) (int, error) {
	return 0, errors.New("not implemented: unused by MemoriesCommandHandler tests")
}

// fakeMemoriesLLM is a memories.LLMService test double capturing the last
// request it received, mirroring internal/usecase/memories/service_test.go's
// own fakeLLM.
type fakeMemoriesLLM struct {
	response *domain.LLMResponse
	err      error
	lastReq  *domain.LLMRequest
}

func (f *fakeMemoriesLLM) Complete(_ context.Context, _ domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func newTestMemoryCell(id, ownerUserID, scene, content string) *memoryv2.MemoryCell {
	now := time.Now()
	return &memoryv2.MemoryCell{
		ID:             id,
		ConversationID: ownerUserID,
		Scene:          scene,
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.5,
		Content:        content,
		Source:         `["msg-1"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func newTestMemoriesHandler(repo memoryv2.MemoryCellRepository, llm memories.LLMService) *MemoriesCommandHandler {
	var opts []memories.Option
	if llm != nil {
		opts = append(opts, memories.WithLLM(llm))
	}
	return NewMemoriesCommandHandler(memories.NewService(repo, opts...))
}

func TestHandleMemoriesCommand_BareShowsHelp(t *testing.T) {
	h := newTestMemoriesHandler(newFakeCellRepo(), nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Memories Commands") {
		t.Errorf("expected help text, got: %s", result)
	}
}

func TestHandleMemoriesCommand_BrowseNoQuery(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	repo.cells["alice-2"] = newTestMemoryCell("alice-2", "alice", "work-notes", "Alice prefers async standups")
	h := newTestMemoriesHandler(repo, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories browse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Found 2 memory cell(s)") {
		t.Errorf("expected both cells listed, got: %s", result)
	}
	if !strings.Contains(result, "trip-planning") || !strings.Contains(result, "work-notes") {
		t.Errorf("expected both scenes present, got: %s", result)
	}
}

func TestHandleMemoriesCommand_BrowseWithQuery(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	repo.cells["alice-2"] = newTestMemoryCell("alice-2", "alice", "work-notes", "Alice prefers async standups")
	h := newTestMemoriesHandler(repo, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories browse window seats")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Found 1 memory cell(s)") {
		t.Errorf("expected exactly one matching cell, got: %s", result)
	}
	if !strings.Contains(result, "trip-planning") {
		t.Errorf("expected matching cell in output, got: %s", result)
	}
	if strings.Contains(result, "work-notes") {
		t.Errorf("did not expect non-matching cell in output, got: %s", result)
	}
}

func TestHandleMemoriesCommand_BrowseNoMatches(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	h := newTestMemoriesHandler(repo, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories browse nonexistent-topic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No memory cells found matching") {
		t.Errorf("expected no-match message, got: %s", result)
	}
}

func TestHandleMemoriesCommand_ChatSuccess(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	llm := &fakeMemoriesLLM{response: &domain.LLMResponse{Content: "Alice prefers a window seat."}}
	h := newTestMemoriesHandler(repo, llm)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories chat alice-1 what seat does she like?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Alice prefers a window seat.") {
		t.Errorf("expected answer in output, got: %s", result)
	}
	if !strings.Contains(result, "what seat does she like?") {
		t.Errorf("expected question echoed in output, got: %s", result)
	}
	if llm.lastReq == nil {
		t.Fatal("expected LLM to be called")
	}
	if len(llm.lastReq.Messages) != 1 || llm.lastReq.Messages[0].Content != "what seat does she like?" {
		t.Errorf("expected multi-word question to be joined and passed through, got: %+v", llm.lastReq.Messages)
	}
}

func TestHandleMemoriesCommand_ChatNonexistentCellID(t *testing.T) {
	repo := newFakeCellRepo()
	llm := &fakeMemoriesLLM{response: &domain.LLMResponse{Content: "should never be reached"}}
	h := newTestMemoriesHandler(repo, llm)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories chat missing-cell what is this?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Memory cell not found: missing-cell") {
		t.Errorf("expected not-found message, got: %s", result)
	}
}

func TestHandleMemoriesCommand_ChatMissingArgsShowsUsage(t *testing.T) {
	h := newTestMemoriesHandler(newFakeCellRepo(), nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories chat alice-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Usage: /memories chat") {
		t.Errorf("expected usage message, got: %s", result)
	}
}

func TestHandleMemoriesCommand_UsesOwnerUserIDNotCurrentUserID(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	h := newTestMemoriesHandler(repo, nil)

	// currentUser.ID is deliberately a different value than ownerUserID
	// (AD-5): the handler must scope by ownerUserID, never currentUser.ID.
	user := &domain.User{ID: "some-other-internal-id", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories browse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "trip-planning") {
		t.Errorf("expected ownerUserID (alice) to scope the browse, got: %s", result)
	}
}

func TestHandleMemoriesCommand_CrossUserIsolation_BrowseDoesNotSeeOtherUsersCells(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	repo.cells["bob-1"] = newTestMemoryCell("bob-1", "bob", "trip-planning", "Bob likes aisle seats")
	h := newTestMemoriesHandler(repo, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories browse")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Alice likes window seats") {
		t.Errorf("expected alice's own cell, got: %s", result)
	}
	if strings.Contains(result, "Bob likes aisle seats") {
		t.Errorf("cross-user isolation violated: bob's cell leaked into alice's browse, got: %s", result)
	}
}

func TestHandleMemoriesCommand_CrossUserIsolation_ChatCannotReachOtherUsersCell(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["bob-1"] = newTestMemoryCell("bob-1", "bob", "trip-planning", "Bob's secret plan")
	llm := &fakeMemoriesLLM{response: &domain.LLMResponse{Content: "should never be reached"}}
	h := newTestMemoriesHandler(repo, llm)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories chat bob-1 tell me the secret plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Memory cell not found: bob-1") {
		t.Errorf("expected not-found (existence not disclosed across owners), got: %s", result)
	}
	if llm.lastReq != nil {
		t.Error("expected LLM never called for a cell belonging to a different owner")
	}
}

func TestHandleMemoriesCommand_UnknownSubcommand(t *testing.T) {
	h := newTestMemoriesHandler(newFakeCellRepo(), nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleMemoriesCommand(context.Background(), user, "alice", "/memories bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Unknown memories command") {
		t.Errorf("expected unknown-command message, got: %s", result)
	}
}

func TestMemoriesCommandHandler_Handle_SatisfiesEnvCommandHandler(t *testing.T) {
	var _ EnvCommandHandler = NewMemoriesCommandHandler(memories.NewService(newFakeCellRepo()))
}
