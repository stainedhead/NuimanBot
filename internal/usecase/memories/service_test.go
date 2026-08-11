package memories

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
)

// fakeCellRepo is an in-memory memoryv2.MemoryCellRepository test double.
// Only Get and List are exercised by Service; the remaining interface
// methods are stubbed since the read-only Memories environment (FR-046)
// never calls them.
type fakeCellRepo struct {
	cells   map[string]*memoryv2.MemoryCell
	getErr  error
	listErr error
}

func newFakeCellRepo() *fakeCellRepo {
	return &fakeCellRepo{cells: make(map[string]*memoryv2.MemoryCell)}
}

func (f *fakeCellRepo) Create(_ context.Context, cell *memoryv2.MemoryCell) error {
	f.cells[cell.ID] = cell
	return nil
}

func (f *fakeCellRepo) Get(_ context.Context, id string) (*memoryv2.MemoryCell, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	cell, ok := f.cells[id]
	if !ok {
		return nil, memoryv2.ErrNotFound
	}
	return cell, nil
}

func (f *fakeCellRepo) List(_ context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
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
	return errors.New("not implemented: read-only Memories environment never calls Update")
}

func (f *fakeCellRepo) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented: read-only Memories environment never calls Delete")
}

func (f *fakeCellRepo) SearchFTS(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, errors.New("not implemented: unused by Service")
}

func (f *fakeCellRepo) GetByScene(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, errors.New("not implemented: unused by Service")
}

func (f *fakeCellRepo) GetHighSalience(_ context.Context, _ string, _ float64, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, errors.New("not implemented: unused by Service")
}

func (f *fakeCellRepo) DeleteExpired(_ context.Context) (int, error) {
	return 0, nil
}

var _ memoryv2.MemoryCellRepository = (*fakeCellRepo)(nil)

func newTestCell(id, ownerUserID, scene, content string) *memoryv2.MemoryCell {
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

func TestListCells_ScopesToOwner_IgnoresTamperedFilter(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	repo.cells["bob-1"] = newTestCell("bob-1", "bob", "trip-planning", "Bob likes aisle seats")

	svc := NewService(repo)

	// A caller-supplied filter that tries to read bob's cells by setting
	// ConversationID directly must be overridden by the service — the
	// service is the isolation enforcement point, not the caller.
	got, err := svc.ListCells(context.Background(), "alice", memoryv2.MemoryCellFilter{ConversationID: "bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "alice-1" {
		t.Fatalf("expected only alice's cell, got %+v", got)
	}
}

func TestListCells_AppliesSceneFilter(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "content 1")
	repo.cells["alice-2"] = newTestCell("alice-2", "alice", "work-notes", "content 2")

	svc := NewService(repo)

	got, err := svc.ListCells(context.Background(), "alice", memoryv2.MemoryCellFilter{Scene: "work-notes"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "alice-2" {
		t.Fatalf("expected only the work-notes cell, got %+v", got)
	}
}

func TestListCells_EmptyOwnerUserID_ReturnsError(t *testing.T) {
	svc := NewService(newFakeCellRepo())
	if _, err := svc.ListCells(context.Background(), "", memoryv2.MemoryCellFilter{}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

func TestListCells_PropagatesRepoError(t *testing.T) {
	repo := newFakeCellRepo()
	repo.listErr = errors.New("boom")
	svc := NewService(repo)
	if _, err := svc.ListCells(context.Background(), "alice", memoryv2.MemoryCellFilter{}); err == nil {
		t.Fatal("expected error to propagate from repo.List")
	}
}

func TestGetCell_OwnCellSucceeds(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	svc := NewService(repo)

	got, err := svc.GetCell(context.Background(), "alice", "alice-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "alice-1" {
		t.Fatalf("expected alice-1, got %+v", got)
	}
}

func TestGetCell_CrossOwnerReturnsNotFound(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["bob-1"] = newTestCell("bob-1", "bob", "trip-planning", "Bob's secret")
	svc := NewService(repo)

	_, err := svc.GetCell(context.Background(), "alice", "bob-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound for cross-owner access (not disclosed), got %v", err)
	}
}

func TestGetCell_NonexistentCellReturnsNotFound(t *testing.T) {
	svc := NewService(newFakeCellRepo())
	_, err := svc.GetCell(context.Background(), "alice", "does-not-exist")
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Fatalf("expected memoryv2.ErrNotFound, got %v", err)
	}
}

func TestGetCell_EmptyOwnerUserID_ReturnsError(t *testing.T) {
	svc := NewService(newFakeCellRepo())
	if _, err := svc.GetCell(context.Background(), "", "some-id"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

// fakeLLM is a memories.LLMService test double capturing the last request it
// received, so tests can assert the per-cell chat is actually grounded in
// the cell's own content (FR-R4's acceptance criteria).
type fakeLLM struct {
	lastReq  *domain.LLMRequest
	response *domain.LLMResponse
	err      error
}

func (f *fakeLLM) Complete(_ context.Context, _ domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func TestAskAboutCell_GroundsSystemPromptInCellContent_ReturnsAnswer(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	llm := &fakeLLM{response: &domain.LLMResponse{Content: "Alice prefers a window seat."}}
	svc := NewService(repo, WithLLM(llm))

	answer, err := svc.AskAboutCell(context.Background(), "alice", "alice-1", "What seat does Alice like?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "Alice prefers a window seat." {
		t.Fatalf("expected the LLM's answer to be returned verbatim, got %q", answer)
	}
	if llm.lastReq == nil {
		t.Fatal("expected the LLM to be called")
	}
	if !strings.Contains(llm.lastReq.SystemPrompt, "Alice likes window seats") {
		t.Fatalf("expected system prompt to be grounded in the cell's content, got %q", llm.lastReq.SystemPrompt)
	}
	if !strings.Contains(llm.lastReq.SystemPrompt, "trip-planning") {
		t.Fatalf("expected system prompt to include the cell's scene, got %q", llm.lastReq.SystemPrompt)
	}
	if len(llm.lastReq.Messages) != 1 || llm.lastReq.Messages[0].Content != "What seat does Alice like?" {
		t.Fatalf("expected the question as the sole user message, got %+v", llm.lastReq.Messages)
	}
}

func TestAskAboutCell_CrossOwnerReturnsNotFound(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["bob-1"] = newTestCell("bob-1", "bob", "trip-planning", "Bob's secret")
	llm := &fakeLLM{response: &domain.LLMResponse{Content: "should never be reached"}}
	svc := NewService(repo, WithLLM(llm))

	_, err := svc.AskAboutCell(context.Background(), "alice", "bob-1", "What is it?")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound for cross-owner access, got %v", err)
	}
	if llm.lastReq != nil {
		t.Fatal("expected the LLM never to be called for a cell the caller doesn't own")
	}
}

func TestAskAboutCell_EmptyQuestion_ReturnsError(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "content")
	svc := NewService(repo, WithLLM(&fakeLLM{}))

	_, err := svc.AskAboutCell(context.Background(), "alice", "alice-1", "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput for an empty question, got %v", err)
	}
}

func TestAskAboutCell_PropagatesLLMError(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "content")
	llm := &fakeLLM{err: errors.New("provider unavailable")}
	svc := NewService(repo, WithLLM(llm))

	_, err := svc.AskAboutCell(context.Background(), "alice", "alice-1", "question")
	if err == nil {
		t.Fatal("expected the LLM error to propagate")
	}
}

func TestAskAboutCell_UsesConfiguredLLMDefaults(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "content")
	llm := &fakeLLM{response: &domain.LLMResponse{Content: "answer"}}
	svc := NewService(repo, WithLLM(llm), WithLLMDefaults(LLMDefaults{
		Model:       "custom-model",
		MaxTokens:   2048,
		Temperature: 0.3,
	}))

	if _, err := svc.AskAboutCell(context.Background(), "alice", "alice-1", "question"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm.lastReq.Model != "custom-model" {
		t.Errorf("expected configured Model to be used, got %q", llm.lastReq.Model)
	}
	if llm.lastReq.MaxTokens != 2048 {
		t.Errorf("expected configured MaxTokens to be used, got %d", llm.lastReq.MaxTokens)
	}
	if llm.lastReq.Temperature != 0.3 {
		t.Errorf("expected configured Temperature to be used, got %v", llm.lastReq.Temperature)
	}
}

func TestAskAboutCell_NoLLMConfigured_ReturnsClearError(t *testing.T) {
	repo := newFakeCellRepo()
	repo.cells["alice-1"] = newTestCell("alice-1", "alice", "trip-planning", "content")
	svc := NewService(repo) // no WithLLM option

	_, err := svc.AskAboutCell(context.Background(), "alice", "alice-1", "question")
	if err == nil {
		t.Fatal("expected an error when no LLM is configured, not a panic or silent no-op")
	}
}
