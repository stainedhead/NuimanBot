package persona

import (
	"context"
	"nuimanbot/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- FileCache Tests ---

func TestNewFileCache(t *testing.T) {
	cache := NewFileCache(15 * time.Minute)
	if cache == nil {
		t.Fatal("NewFileCache returned nil")
	}
	if cache.ttl != 15*time.Minute {
		t.Errorf("expected TTL 15m, got %v", cache.ttl)
	}
}

func TestFileCache_SetAndGet(t *testing.T) {
	cache := NewFileCache(15 * time.Minute)
	now := time.Now()

	cache.Set("key1", "content1", now, 8)

	entry, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected cache hit for key1")
	}
	if entry.Content != "content1" {
		t.Errorf("expected content1, got %s", entry.Content)
	}
	if entry.ModifiedAt != now {
		t.Errorf("expected modifiedAt %v, got %v", now, entry.ModifiedAt)
	}
	if entry.SizeBytes != 8 {
		t.Errorf("expected sizeBytes 8, got %d", entry.SizeBytes)
	}
}

func TestFileCache_GetMiss(t *testing.T) {
	cache := NewFileCache(15 * time.Minute)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestFileCache_Expiration(t *testing.T) {
	cache := NewFileCache(1 * time.Millisecond)

	cache.Set("key1", "content1", time.Now(), 8)
	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
}

func TestFileCache_Delete(t *testing.T) {
	cache := NewFileCache(15 * time.Minute)

	cache.Set("key1", "content1", time.Now(), 8)
	cache.Delete("key1")

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected cache miss after delete")
	}
}

func TestFileCache_DeletePrefix(t *testing.T) {
	cache := NewFileCache(15 * time.Minute)
	now := time.Now()

	cache.Set("user1:SOUL", "soul content", now, 12)
	cache.Set("user1:USER", "user content", now, 12)
	cache.Set("user2:SOUL", "other soul", now, 10)

	cache.DeletePrefix("user1:")

	_, ok1 := cache.Get("user1:SOUL")
	_, ok2 := cache.Get("user1:USER")
	_, ok3 := cache.Get("user2:SOUL")

	if ok1 {
		t.Error("expected user1:SOUL deleted")
	}
	if ok2 {
		t.Error("expected user1:USER deleted")
	}
	if !ok3 {
		t.Error("expected user2:SOUL to remain")
	}
}

// --- FileRepository Constructor Tests ---

func newTestRepo(t *testing.T) (*FileRepository, string) {
	t.Helper()
	tmpDir := t.TempDir()
	repo := NewFileRepository(tmpDir)
	return repo, tmpDir
}

func TestNewFileRepository(t *testing.T) {
	repo := NewFileRepository("/tmp/test")
	if repo == nil {
		t.Fatal("NewFileRepository returned nil")
	}
	if repo.basePath != "/tmp/test" {
		t.Errorf("expected basePath /tmp/test, got %s", repo.basePath)
	}
}

// --- FileRepository.Save Tests ---

func TestFileRepository_Save_NewFile(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	file := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       filepath.Join(tmpDir, "user-1", "SOUL.md"),
		Content:    "# My Soul\n\nI am a helpful assistant.",
		ModifiedAt: time.Now(),
		SizeBytes:  35,
	}

	err := repo.Save(ctx, file)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists on disk
	diskPath := filepath.Join(tmpDir, "user-1", "SOUL.md")
	data, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatalf("file not found on disk: %v", err)
	}
	if string(data) != file.Content {
		t.Errorf("content mismatch: expected %q, got %q", file.Content, string(data))
	}
}

func TestFileRepository_Save_UpdateExisting(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	file := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       filepath.Join(tmpDir, "user-1", "SOUL.md"),
		Content:    "# Original",
		ModifiedAt: time.Now(),
		SizeBytes:  10,
	}

	if err := repo.Save(ctx, file); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	file.Content = "# Updated"
	file.ModifiedAt = time.Now()

	if err := repo.Save(ctx, file); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "user-1", "SOUL.md"))
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != "# Updated" {
		t.Errorf("expected updated content, got %q", string(data))
	}
}

func TestFileRepository_Save_InvalidFile(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	file := &domain.PersonaFile{
		UserID:     "",
		Type:       domain.PersonaFileSOUL,
		Path:       "/tmp/SOUL.md",
		Content:    "content",
		ModifiedAt: time.Now(),
	}

	err := repo.Save(ctx, file)
	if err == nil {
		t.Fatal("expected error for invalid file (empty UserID)")
	}
}

func TestFileRepository_Save_ContentTooLarge(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	largeContent := strings.Repeat("x", domain.MaxPersonaFileSize+1)
	file := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       filepath.Join(tmpDir, "user-1", "SOUL.md"),
		Content:    largeContent,
		ModifiedAt: time.Now(),
		SizeBytes:  int64(len(largeContent)),
	}

	err := repo.Save(ctx, file)
	if err == nil {
		t.Fatal("expected error for content too large")
	}
}

func TestFileRepository_Save_PathTraversal(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	file := &domain.PersonaFile{
		UserID:     "../../../etc",
		Type:       domain.PersonaFileSOUL,
		Path:       filepath.Join(tmpDir, "../../../etc", "SOUL.md"),
		Content:    "evil",
		ModifiedAt: time.Now(),
		SizeBytes:  4,
	}

	err := repo.Save(ctx, file)
	if err == nil {
		t.Fatal("expected error for path traversal in save")
	}
}

// --- FileRepository.Get Tests ---

func TestFileRepository_Get_Existing(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	userDir := filepath.Join(tmpDir, "user-1")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "# My Soul"
	if err := os.WriteFile(filepath.Join(userDir, "SOUL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := repo.Get(ctx, "user-1", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if file.UserID != "user-1" {
		t.Errorf("expected UserID user-1, got %s", file.UserID)
	}
	if file.Type != domain.PersonaFileSOUL {
		t.Errorf("expected type SOUL, got %s", file.Type)
	}
	if file.Content != content {
		t.Errorf("expected content %q, got %q", content, file.Content)
	}
	if file.Path != filepath.Join(userDir, "SOUL.md") {
		t.Errorf("expected path %s, got %s", filepath.Join(userDir, "SOUL.md"), file.Path)
	}
	if file.ModifiedAt.IsZero() {
		t.Error("expected non-zero ModifiedAt")
	}
	if file.SizeBytes != int64(len(content)) {
		t.Errorf("expected SizeBytes %d, got %d", len(content), file.SizeBytes)
	}
}

func TestFileRepository_Get_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "user-nonexistent", domain.PersonaFileSOUL)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if err != domain.ErrPersonaFileNotFound {
		t.Errorf("expected ErrPersonaFileNotFound, got %v", err)
	}
}

func TestFileRepository_Get_InvalidType(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "user-1", domain.PersonaFileType(99))
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestFileRepository_Get_EmptyUserID(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "", domain.PersonaFileSOUL)
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
}

func TestFileRepository_Get_PathTraversal(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "../../../etc", domain.PersonaFileSOUL)
	if err == nil {
		t.Fatal("expected error for path traversal in userID")
	}
}

func TestFileRepository_Get_UsesCache(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	userDir := filepath.Join(tmpDir, "user-1")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "SOUL.md"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	// First read: populates cache
	file1, err := repo.Get(ctx, "user-1", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("first Get failed: %v", err)
	}
	if file1.Content != "original" {
		t.Errorf("expected original, got %s", file1.Content)
	}

	// Modify file on disk directly (bypassing repository)
	if err := os.WriteFile(filepath.Join(userDir, "SOUL.md"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second read: should return cached version
	file2, err := repo.Get(ctx, "user-1", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if file2.Content != "original" {
		t.Errorf("expected cached original, got %s", file2.Content)
	}
}

func TestFileRepository_Save_InvalidatesCache(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	file := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       filepath.Join(tmpDir, "user-1", "SOUL.md"),
		Content:    "original",
		ModifiedAt: time.Now(),
		SizeBytes:  8,
	}

	// Save and Get to populate cache
	if err := repo.Save(ctx, file); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	got, err := repo.Get(ctx, "user-1", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Content != "original" {
		t.Fatalf("expected original, got %s", got.Content)
	}

	// Save updated version (should invalidate cache)
	file.Content = "updated"
	file.ModifiedAt = time.Now()
	if err := repo.Save(ctx, file); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	// Get should return updated content (reads from disk after cache invalidation)
	got, err = repo.Get(ctx, "user-1", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("get after update failed: %v", err)
	}
	if got.Content != "updated" {
		t.Errorf("expected updated, got %s", got.Content)
	}
}

// --- FileRepository.Delete Tests ---

func TestFileRepository_Delete_Existing(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	userDir := filepath.Join(tmpDir, "user-1")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "SOUL.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := repo.Delete(ctx, "user-1", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(filepath.Join(userDir, "SOUL.md")); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestFileRepository_Delete_NonExistent(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	// Delete should be idempotent
	err := repo.Delete(ctx, "user-nonexistent", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("Delete of non-existent should not error, got: %v", err)
	}
}

func TestFileRepository_Delete_EmptyUserID(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "", domain.PersonaFileSOUL)
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
}

func TestFileRepository_Delete_PathTraversal(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "../../../etc", domain.PersonaFileSOUL)
	if err == nil {
		t.Fatal("expected error for path traversal in delete")
	}
}

func TestFileRepository_Delete_InvalidatesCache(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	file := &domain.PersonaFile{
		UserID:     "user-1",
		Type:       domain.PersonaFileSOUL,
		Path:       filepath.Join(tmpDir, "user-1", "SOUL.md"),
		Content:    "cached content",
		ModifiedAt: time.Now(),
		SizeBytes:  14,
	}
	if err := repo.Save(ctx, file); err != nil {
		t.Fatal(err)
	}
	// Populate cache
	if _, err := repo.Get(ctx, "user-1", domain.PersonaFileSOUL); err != nil {
		t.Fatal(err)
	}

	// Delete
	if err := repo.Delete(ctx, "user-1", domain.PersonaFileSOUL); err != nil {
		t.Fatal(err)
	}

	// Get should return not found
	_, err := repo.Get(ctx, "user-1", domain.PersonaFileSOUL)
	if err != domain.ErrPersonaFileNotFound {
		t.Errorf("expected ErrPersonaFileNotFound after delete, got %v", err)
	}
}

// --- FileRepository.List Tests ---

func TestFileRepository_List_MultipleFiles(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	userDir := filepath.Join(tmpDir, "user-1")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "SOUL.md"), []byte("soul"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "USER.md"), []byte("user"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := repo.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	types := make(map[domain.PersonaFileType]bool)
	for _, f := range files {
		types[f.Type] = true
	}
	if !types[domain.PersonaFileSOUL] {
		t.Error("expected SOUL file in list")
	}
	if !types[domain.PersonaFileUSER] {
		t.Error("expected USER file in list")
	}
}

func TestFileRepository_List_NoFiles(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	files, err := repo.List(ctx, "user-nonexistent")
	if err != nil {
		t.Fatalf("List should not error for missing user, got: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty slice, got %d files", len(files))
	}
}

func TestFileRepository_List_AllThreeTypes(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	userDir := filepath.Join(tmpDir, "user-1")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SOUL.md", "USER.md", "RULES.md"} {
		if err := os.WriteFile(filepath.Join(userDir, name), []byte("content of "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := repo.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
}

func TestFileRepository_List_EmptyUserID(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	_, err := repo.List(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
}

// --- FileRepository Round-Trip Test ---

func TestFileRepository_RoundTrip(t *testing.T) {
	repo, tmpDir := newTestRepo(t)
	ctx := context.Background()

	original := &domain.PersonaFile{
		UserID:     "user-roundtrip",
		Type:       domain.PersonaFileRULES,
		Path:       filepath.Join(tmpDir, "user-roundtrip", "RULES.md"),
		Content:    "---\nblocked_tools:\n  - shell.exec\n---\n# Rules\nBe safe.",
		ModifiedAt: time.Now(),
		SizeBytes:  55,
	}

	if err := repo.Save(ctx, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	retrieved, err := repo.Get(ctx, "user-roundtrip", domain.PersonaFileRULES)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Content != original.Content {
		t.Errorf("content mismatch after roundtrip")
	}
	if retrieved.UserID != original.UserID {
		t.Errorf("userID mismatch: expected %s, got %s", original.UserID, retrieved.UserID)
	}
	if retrieved.Type != original.Type {
		t.Errorf("type mismatch: expected %s, got %s", original.Type, retrieved.Type)
	}
}

// --- Interface Compliance Test ---

func TestFileRepository_ImplementsInterface(t *testing.T) {
	var _ domain.PersonaFileRepository = (*FileRepository)(nil)
}
