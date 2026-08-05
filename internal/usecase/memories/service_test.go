package memories

import (
	"context"
	"errors"
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
