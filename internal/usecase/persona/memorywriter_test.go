package persona_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/persona"
)

// --- Mocks ---

// mockPersonaFileRepo implements domain.PersonaFileRepository for testing.
type mockPersonaFileRepo struct {
	getFunc    func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error)
	saveFunc   func(ctx context.Context, file *domain.PersonaFile) error
	deleteFunc func(ctx context.Context, userID string, fileType domain.PersonaFileType) error
	listFunc   func(ctx context.Context, userID string) ([]*domain.PersonaFile, error)
	saved      []*domain.PersonaFile
}

func (m *mockPersonaFileRepo) Get(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, userID, fileType)
	}
	return nil, domain.ErrPersonaFileNotFound
}

func (m *mockPersonaFileRepo) Save(ctx context.Context, file *domain.PersonaFile) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, file)
	}
	m.saved = append(m.saved, file)
	return nil
}

func (m *mockPersonaFileRepo) Delete(ctx context.Context, userID string, fileType domain.PersonaFileType) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, userID, fileType)
	}
	return nil
}

func (m *mockPersonaFileRepo) List(ctx context.Context, userID string) ([]*domain.PersonaFile, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, userID)
	}
	return nil, nil
}

// mockAuditLogger implements persona.AuditLogger for testing.
type mockAuditLogger struct {
	entries []persona.AuditEntry
	err     error
}

func (m *mockAuditLogger) Log(entry persona.AuditEntry) error {
	if m.err != nil {
		return m.err
	}
	m.entries = append(m.entries, entry)
	return nil
}

// mockRulesChecker implements persona.RulesChecker for testing.
type mockRulesChecker struct {
	allowed           bool
	reason            string
	needsConfirmation bool
	isAllowedErr      error
	needsConfirmErr   error
}

func (m *mockRulesChecker) IsAllowed(ctx context.Context, userID, action string) (bool, string, error) {
	return m.allowed, m.reason, m.isAllowedErr
}

func (m *mockRulesChecker) NeedsConfirmation(ctx context.Context, userID, action string) (bool, error) {
	return m.needsConfirmation, m.needsConfirmErr
}

// --- Helpers ---

func newAllowedRulesChecker() *mockRulesChecker {
	return &mockRulesChecker{allowed: true}
}

func newTestWriter(repo domain.PersonaFileRepository, auditor persona.AuditLogger, checker persona.RulesChecker) *persona.MemoryWriter {
	return persona.NewMemoryWriter(repo, auditor, checker)
}

// --- Constructor Tests ---

func TestNewMemoryWriter(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	auditor := &mockAuditLogger{}
	checker := newAllowedRulesChecker()

	w := persona.NewMemoryWriter(repo, auditor, checker)
	if w == nil {
		t.Fatal("NewMemoryWriter returned nil")
	}
}

// --- Input Validation Tests ---

func TestWrite_EmptyUserID(t *testing.T) {
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "",
		FilePath:  "SOUL.md",
		Content:   "hello",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
	if !strings.Contains(err.Error(), "userID") {
		t.Errorf("error should mention userID, got: %v", err)
	}
}

func TestWrite_EmptyFilePath(t *testing.T) {
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "",
		Content:   "hello",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error for empty filePath")
	}
	if !strings.Contains(err.Error(), "filePath") {
		t.Errorf("error should mention filePath, got: %v", err)
	}
}

func TestWrite_EmptyContent(t *testing.T) {
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("error should mention content, got: %v", err)
	}
}

func TestWrite_InvalidOperation(t *testing.T) {
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "hello",
		Operation: "delete",
	})
	if err == nil {
		t.Fatal("expected error for invalid operation")
	}
	if !strings.Contains(err.Error(), "operation") {
		t.Errorf("error should mention operation, got: %v", err)
	}
}

func TestWrite_InvalidFilePath(t *testing.T) {
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "UNKNOWN.md",
		Content:   "hello",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error for invalid file path")
	}
	if !strings.Contains(err.Error(), "unsupported persona file") {
		t.Errorf("error should mention unsupported file, got: %v", err)
	}
}

// --- Replace Operation Tests ---

func TestWrite_Replace_Success(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	auditor := &mockAuditLogger{}
	w := newTestWriter(repo, auditor, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "new soul content",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true")
	}
	if out.RequiresConfirmation {
		t.Error("should not require confirmation")
	}

	// Verify the file was saved
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 save call, got %d", len(repo.saved))
	}
	saved := repo.saved[0]
	if saved.UserID != "user-1" {
		t.Errorf("expected userID 'user-1', got '%s'", saved.UserID)
	}
	if saved.Type != domain.PersonaFileSOUL {
		t.Errorf("expected type SOUL, got %v", saved.Type)
	}
	if saved.Content != "new soul content" {
		t.Errorf("expected content 'new soul content', got '%s'", saved.Content)
	}
}

func TestWrite_Replace_USER(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "USER.md",
		Content:   "user preferences",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true")
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(repo.saved))
	}
	if repo.saved[0].Type != domain.PersonaFileUSER {
		t.Errorf("expected type USER, got %v", repo.saved[0].Type)
	}
}

func TestWrite_Replace_RULES(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "RULES.md",
		Content:   "rules content",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true")
	}
	if repo.saved[0].Type != domain.PersonaFileRULES {
		t.Errorf("expected type RULES, got %v", repo.saved[0].Type)
	}
}

// --- Append Operation Tests ---

func TestWrite_Append_ToExistingFile(t *testing.T) {
	existing := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       "/data/user-1/SOUL.md",
		Content:    "existing content",
		ModifiedAt: time.Now(),
		SizeBytes:  16,
	}
	repo := &mockPersonaFileRepo{
		getFunc: func(_ context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			if userID == "user-1" && fileType == domain.PersonaFileSOUL {
				return existing, nil
			}
			return nil, domain.ErrPersonaFileNotFound
		},
	}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "appended content",
		Operation: "append",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true")
	}

	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(repo.saved))
	}
	expected := "existing content\nappended content"
	if repo.saved[0].Content != expected {
		t.Errorf("expected content %q, got %q", expected, repo.saved[0].Content)
	}
}

func TestWrite_Append_ToNewFile(t *testing.T) {
	repo := &mockPersonaFileRepo{} // Default getFunc returns ErrPersonaFileNotFound
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "first content",
		Operation: "append",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true")
	}

	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(repo.saved))
	}
	if repo.saved[0].Content != "first content" {
		t.Errorf("expected content 'first content', got '%s'", repo.saved[0].Content)
	}
}

func TestWrite_Append_ToEmptyFile(t *testing.T) {
	existing := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       "/data/user-1/SOUL.md",
		Content:    "   ",
		ModifiedAt: time.Now(),
		SizeBytes:  3,
	}
	repo := &mockPersonaFileRepo{
		getFunc: func(_ context.Context, _ string, _ domain.PersonaFileType) (*domain.PersonaFile, error) {
			return existing, nil
		},
	}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "new content",
		Operation: "append",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true")
	}
	if repo.saved[0].Content != "new content" {
		t.Errorf("expected content 'new content', got '%s'", repo.saved[0].Content)
	}
}

// --- Rules Enforcement Tests ---

func TestWrite_ActionBlocked(t *testing.T) {
	checker := &mockRulesChecker{
		allowed: false,
		reason:  "memory writes are blocked",
	}
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, checker)

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "hello",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error when action is blocked")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error should mention blocked, got: %v", err)
	}
}

func TestWrite_RequiresConfirmation(t *testing.T) {
	checker := &mockRulesChecker{
		allowed:           true,
		needsConfirmation: true,
	}
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, checker)

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "hello",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Success {
		t.Error("expected Success to be false when confirmation required")
	}
	if !out.RequiresConfirmation {
		t.Error("expected RequiresConfirmation to be true")
	}
	if out.ConfirmationID == "" {
		t.Error("expected ConfirmationID to be non-empty")
	}
}

func TestWrite_RulesCheckError(t *testing.T) {
	checker := &mockRulesChecker{
		isAllowedErr: errors.New("rules service unavailable"),
	}
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, checker)

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "hello",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error when rules check fails")
	}
	if !strings.Contains(err.Error(), "rules check") {
		t.Errorf("error should mention rules check, got: %v", err)
	}
}

func TestWrite_ConfirmationCheckError(t *testing.T) {
	checker := &mockRulesChecker{
		allowed:         true,
		needsConfirmErr: errors.New("confirmation service error"),
	}
	w := newTestWriter(&mockPersonaFileRepo{}, &mockAuditLogger{}, checker)

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "hello",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error when confirmation check fails")
	}
	if !strings.Contains(err.Error(), "confirmation check") {
		t.Errorf("error should mention confirmation check, got: %v", err)
	}
}

// --- Audit Logging Tests ---

func TestWrite_AuditLogged(t *testing.T) {
	auditor := &mockAuditLogger{}
	w := newTestWriter(&mockPersonaFileRepo{}, auditor, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(auditor.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditor.entries))
	}

	entry := auditor.entries[0]
	if entry.UserID != "user-1" {
		t.Errorf("expected audit userID 'user-1', got '%s'", entry.UserID)
	}
	if entry.Action != "persona_replace" {
		t.Errorf("expected audit action 'persona_replace', got '%s'", entry.Action)
	}
	if entry.FilePath != "SOUL.md" {
		t.Errorf("expected audit filePath 'SOUL.md', got '%s'", entry.FilePath)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero audit timestamp")
	}
	if entry.Details["operation"] != "replace" {
		t.Errorf("expected details.operation 'replace', got '%v'", entry.Details["operation"])
	}
	if entry.Details["file_type"] != "SOUL" {
		t.Errorf("expected details.file_type 'SOUL', got '%v'", entry.Details["file_type"])
	}
}

func TestWrite_AuditAppendAction(t *testing.T) {
	auditor := &mockAuditLogger{}
	w := newTestWriter(&mockPersonaFileRepo{}, auditor, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "USER.md",
		Content:   "content",
		Operation: "append",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if auditor.entries[0].Action != "persona_append" {
		t.Errorf("expected audit action 'persona_append', got '%s'", auditor.entries[0].Action)
	}
}

func TestWrite_AuditFailureNonBlocking(t *testing.T) {
	auditor := &mockAuditLogger{err: errors.New("audit write failed")}
	repo := &mockPersonaFileRepo{}
	w := newTestWriter(repo, auditor, newAllowedRulesChecker())

	out, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("audit failure should not block write, got error: %v", err)
	}
	if !out.Success {
		t.Error("expected Success to be true despite audit failure")
	}
	if len(repo.saved) != 1 {
		t.Errorf("file should still be saved, got %d saves", len(repo.saved))
	}
}

// --- Repository Error Tests ---

func TestWrite_RepoSaveError(t *testing.T) {
	repo := &mockPersonaFileRepo{
		saveFunc: func(_ context.Context, _ *domain.PersonaFile) error {
			return errors.New("disk full")
		},
	}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "replace",
	})
	if err == nil {
		t.Fatal("expected error on repo save failure")
	}
	if !strings.Contains(err.Error(), "save") {
		t.Errorf("error should mention save, got: %v", err)
	}
}

func TestWrite_Append_RepoGetError(t *testing.T) {
	repo := &mockPersonaFileRepo{
		getFunc: func(_ context.Context, _ string, _ domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, errors.New("read error")
		},
	}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "append",
	})
	if err == nil {
		t.Fatal("expected error on repo get failure during append")
	}
	if !strings.Contains(err.Error(), "read existing") {
		t.Errorf("error should mention read existing, got: %v", err)
	}
}

// --- No Audit or Save When Blocked ---

func TestWrite_BlockedNoSaveOrAudit(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	auditor := &mockAuditLogger{}
	checker := &mockRulesChecker{allowed: false, reason: "blocked"}
	w := newTestWriter(repo, auditor, checker)

	_, _ = w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "replace",
	})

	if len(repo.saved) != 0 {
		t.Errorf("no files should be saved when blocked, got %d saves", len(repo.saved))
	}
	if len(auditor.entries) != 0 {
		t.Errorf("no audit entries should exist when blocked, got %d entries", len(auditor.entries))
	}
}

// --- No Save When Confirmation Required ---

func TestWrite_ConfirmationNoSave(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	checker := &mockRulesChecker{allowed: true, needsConfirmation: true}
	w := newTestWriter(repo, &mockAuditLogger{}, checker)

	_, _ = w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "replace",
	})

	if len(repo.saved) != 0 {
		t.Errorf("no files should be saved when confirmation required, got %d saves", len(repo.saved))
	}
}

// --- PersonaFile Metadata Tests ---

func TestWrite_SetsModifiedAt(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	before := time.Now()
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   "content",
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := time.Now()

	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(repo.saved))
	}
	mod := repo.saved[0].ModifiedAt
	if mod.Before(before) || mod.After(after) {
		t.Errorf("ModifiedAt %v should be between %v and %v", mod, before, after)
	}
}

func TestWrite_SetsSizeBytes(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	w := newTestWriter(repo, &mockAuditLogger{}, newAllowedRulesChecker())

	content := "hello world"
	_, err := w.Write(context.Background(), persona.WriteInput{
		UserID:    "user-1",
		FilePath:  "SOUL.md",
		Content:   content,
		Operation: "replace",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.saved[0].SizeBytes != int64(len(content)) {
		t.Errorf("expected SizeBytes %d, got %d", len(content), repo.saved[0].SizeBytes)
	}
}
