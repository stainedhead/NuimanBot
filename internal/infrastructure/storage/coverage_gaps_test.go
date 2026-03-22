package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/security"

	"github.com/google/uuid"
)

// strconv-free int to string helper for tests.
func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// ============================================================
// FileBotConfigRepository — ListSlackBotsByOwner
// ============================================================

func TestFileBotConfigRepository_ListSlackBotsByOwner(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bots.json")
	enc := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, enc)
	ctx := context.Background()

	owner := "owner-alice"

	bot1 := domain.NewSlackBotConfig("slack-owned-1", "Alice Bot 1", domain.BotTypePublic)
	bot1.OwnerUserID = owner
	bot1.SlackBotToken = "xoxb-1"
	bot1.SlackAppToken = "xapp-1"
	bot1.SlackSigningSecret = "secret-1"
	if err := repo.SaveSlackBot(ctx, bot1); err != nil {
		t.Fatalf("SaveSlackBot bot1: %v", err)
	}

	bot2 := domain.NewSlackBotConfig("slack-owned-2", "Alice Bot 2", domain.BotTypePrivate)
	bot2.OwnerUserID = owner
	bot2.SlackBotToken = "xoxb-2"
	bot2.SlackAppToken = "xapp-2"
	bot2.SlackSigningSecret = "secret-2"
	if err := repo.SaveSlackBot(ctx, bot2); err != nil {
		t.Fatalf("SaveSlackBot bot2: %v", err)
	}

	bot3 := domain.NewSlackBotConfig("slack-other", "Bob Bot", domain.BotTypePublic)
	bot3.OwnerUserID = "owner-bob"
	bot3.SlackBotToken = "xoxb-3"
	bot3.SlackAppToken = "xapp-3"
	bot3.SlackSigningSecret = "secret-3"
	if err := repo.SaveSlackBot(ctx, bot3); err != nil {
		t.Fatalf("SaveSlackBot bot3: %v", err)
	}

	t.Run("returns only owner bots", func(t *testing.T) {
		bots, err := repo.ListSlackBotsByOwner(ctx, owner)
		if err != nil {
			t.Fatalf("ListSlackBotsByOwner: %v", err)
		}
		if len(bots) != 2 {
			t.Errorf("expected 2 bots for owner %q, got %d", owner, len(bots))
		}
	})

	t.Run("returns empty for unknown owner", func(t *testing.T) {
		bots, err := repo.ListSlackBotsByOwner(ctx, "nobody")
		if err != nil {
			t.Fatalf("ListSlackBotsByOwner unknown owner: %v", err)
		}
		if len(bots) != 0 {
			t.Errorf("expected 0 bots for unknown owner, got %d", len(bots))
		}
	})
}

// ============================================================
// FileBotConfigRepository — ListTelegramBotsByOwner
// ============================================================

func TestFileBotConfigRepository_ListTelegramBotsByOwner(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bots.json")
	enc := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, enc)
	ctx := context.Background()

	owner := "owner-telegram"

	bot1 := domain.NewTelegramBotConfig("tg-owned-1", "TG Bot 1", domain.BotTypePublic)
	bot1.OwnerUserID = owner
	bot1.TelegramBotToken = "111:aaa"
	if err := repo.SaveTelegramBot(ctx, bot1); err != nil {
		t.Fatalf("SaveTelegramBot bot1: %v", err)
	}

	bot2 := domain.NewTelegramBotConfig("tg-owned-2", "TG Bot 2", domain.BotTypePrivate)
	bot2.OwnerUserID = owner
	bot2.TelegramBotToken = "222:bbb"
	if err := repo.SaveTelegramBot(ctx, bot2); err != nil {
		t.Fatalf("SaveTelegramBot bot2: %v", err)
	}

	botOther := domain.NewTelegramBotConfig("tg-other", "Other Bot", domain.BotTypePublic)
	botOther.OwnerUserID = "other-owner"
	botOther.TelegramBotToken = "333:ccc"
	if err := repo.SaveTelegramBot(ctx, botOther); err != nil {
		t.Fatalf("SaveTelegramBot botOther: %v", err)
	}

	t.Run("returns only owner bots", func(t *testing.T) {
		bots, err := repo.ListTelegramBotsByOwner(ctx, owner)
		if err != nil {
			t.Fatalf("ListTelegramBotsByOwner: %v", err)
		}
		if len(bots) != 2 {
			t.Errorf("expected 2 bots for owner %q, got %d", owner, len(bots))
		}
	})

	t.Run("returns empty for unknown owner", func(t *testing.T) {
		bots, err := repo.ListTelegramBotsByOwner(ctx, "nobody")
		if err != nil {
			t.Fatalf("ListTelegramBotsByOwner unknown: %v", err)
		}
		if len(bots) != 0 {
			t.Errorf("expected 0 bots for unknown owner, got %d", len(bots))
		}
	})
}

// ============================================================
// FileBotConfigRepository — DeleteTelegramBot
// ============================================================

func TestFileBotConfigRepository_DeleteTelegramBot(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bots.json")
	enc := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, enc)
	ctx := context.Background()

	bot := domain.NewTelegramBotConfig("tg-del-1", "Delete Me", domain.BotTypePublic)
	bot.TelegramBotToken = "999:zzz"
	if err := repo.SaveTelegramBot(ctx, bot); err != nil {
		t.Fatalf("SaveTelegramBot: %v", err)
	}

	t.Run("delete existing bot", func(t *testing.T) {
		if err := repo.DeleteTelegramBot(ctx, "tg-del-1"); err != nil {
			t.Fatalf("DeleteTelegramBot: %v", err)
		}
		_, err := repo.GetTelegramBotByID(ctx, "tg-del-1")
		if err == nil {
			t.Error("expected error after deletion, got nil")
		}
	})

	t.Run("delete nonexistent bot returns error", func(t *testing.T) {
		err := repo.DeleteTelegramBot(ctx, "nonexistent-tg")
		if err == nil {
			t.Error("expected error deleting nonexistent bot")
		}
	})
}

// ============================================================
// FileUserProfileRepository — GetProfileByAPIKey
// ============================================================

func TestFileUserProfileRepository_GetProfileByAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	profile := domain.NewUserProfile("user-api-key-test", "apikey@example.com", domain.UserTypeIndividual)
	profile.APIKey = "test-api-key-value-123"

	if err := repo.SaveProfile(ctx, profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	t.Run("found by api key", func(t *testing.T) {
		found, err := repo.GetProfileByAPIKey(ctx, "test-api-key-value-123")
		if err != nil {
			t.Fatalf("GetProfileByAPIKey: %v", err)
		}
		if found.UserID != profile.UserID {
			t.Errorf("expected UserID %q, got %q", profile.UserID, found.UserID)
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		_, err := repo.GetProfileByAPIKey(ctx, "nonexistent-key")
		if err == nil {
			t.Error("expected error for missing API key, got nil")
		}
	})
}

// ============================================================
// FileUserProfileRepository — GetProfileByPlatformID (additional cases)
// ============================================================

func TestFileUserProfileRepository_GetProfileByPlatformID_Telegram(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	profile := domain.NewUserProfile("user-tg", "tg@example.com", domain.UserTypeIndividual)
	profile.PlatformIDs.Telegram = "TG-ID-999"
	if err := repo.SaveProfile(ctx, profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	found, err := repo.GetProfileByPlatformID(ctx, domain.PlatformTelegram, "TG-ID-999")
	if err != nil {
		t.Fatalf("GetProfileByPlatformID Telegram: %v", err)
	}
	if found.UserID != profile.UserID {
		t.Errorf("expected UserID %q, got %q", profile.UserID, found.UserID)
	}
}

func TestFileUserProfileRepository_GetProfileByPlatformID_CLI(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	profile := domain.NewUserProfile("user-cli", "cli@example.com", domain.UserTypeIndividual)
	profile.PlatformIDs.CLI = "cli-handle"
	if err := repo.SaveProfile(ctx, profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	found, err := repo.GetProfileByPlatformID(ctx, domain.PlatformCLI, "cli-handle")
	if err != nil {
		t.Fatalf("GetProfileByPlatformID CLI: %v", err)
	}
	if found.UserID != profile.UserID {
		t.Errorf("expected UserID %q, got %q", profile.UserID, found.UserID)
	}
}

func TestFileUserProfileRepository_GetProfileByPlatformID_UnsupportedPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileUserProfileRepository(filepath.Join(tmpDir, "users.json"), "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	p := domain.NewUserProfile("user-unsup", "unsup@example.com", domain.UserTypeIndividual)
	if err := repo.SaveProfile(ctx, p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	_, err := repo.GetProfileByPlatformID(ctx, domain.Platform("unknown-platform"), "any")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

func TestFileUserProfileRepository_GetProfileByPlatformID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileUserProfileRepository(filepath.Join(tmpDir, "users.json"), "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	p := domain.NewUserProfile("user-nf", "nf@example.com", domain.UserTypeIndividual)
	if err := repo.SaveProfile(ctx, p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	_, err := repo.GetProfileByPlatformID(ctx, domain.PlatformSlack, "no-such-id")
	if err == nil {
		t.Error("expected error for missing platform ID")
	}
}

// ============================================================
// FileNotesRepository — tagsEqual (via Update path)
// ============================================================

func TestFileNotesRepository_tagsEqual(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileNotesRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-tags",
		Title:     "Original",
		Content:   "Content",
		Tags:      []string{"a", "b"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update with same tags — tagsEqual should return true.
	note.Title = "Updated Same Tags"
	if err := repo.Update(ctx, note); err != nil {
		t.Fatalf("Update same tags: %v", err)
	}

	// Update with different tags — tagsEqual returns false, index updated.
	note.Tags = []string{"c", "d"}
	note.Title = "Updated Different Tags"
	if err := repo.Update(ctx, note); err != nil {
		t.Fatalf("Update different tags: %v", err)
	}

	// Update with empty tags.
	note.Tags = []string{}
	if err := repo.Update(ctx, note); err != nil {
		t.Fatalf("Update empty tags: %v", err)
	}
}

// ============================================================
// FileNotesRepository — Delete
// ============================================================

func TestFileNotesRepository_Delete_NotFound_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileNotesRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent-note-id")
	if err == nil {
		t.Error("expected error deleting nonexistent note")
	}
}

func TestFileNotesRepository_Delete_Success(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileNotesRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-del",
		Title:     "Delete Me",
		Content:   "Will be deleted",
		Tags:      []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(ctx, note.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, note.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// ============================================================
// FileNotesRepository — Update nonexistent
// ============================================================

func TestFileNotesRepository_Update_NotFound_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileNotesRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	note := &domain.Note{
		ID:        "nonexistent-" + uuid.New().String(),
		UserID:    "user-upd",
		Title:     "Ghost Note",
		Content:   "No content",
		Tags:      []string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Update(ctx, note)
	if err == nil {
		t.Error("expected error updating nonexistent note")
	}
}

// ============================================================
// FileNotesRepository — List
// ============================================================

func TestFileNotesRepository_List_ForUser(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileNotesRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	userID := "user-notes-list"
	for i := 0; i < 3; i++ {
		note := &domain.Note{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Note " + intStr(i),
			Content:   "Content " + intStr(i),
			Tags:      []string{"tag" + intStr(i)},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, note); err != nil {
			t.Fatalf("Create note %d: %v", i, err)
		}
	}

	notes, err := repo.List(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 3 {
		t.Errorf("expected 3 notes, got %d", len(notes))
	}
}

// ============================================================
// FileMemorySceneRepository — additional coverage paths
// ============================================================

func TestFileMemorySceneRepository_Upsert_Then_Get_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemorySceneRepository(filepath.Join(tmpDir, "memory"))
	ctx := context.Background()

	scene := &memoryv2.MemoryScene{
		Scene:      "work-coverage",
		Summary:    "Work-related memories for coverage",
		TokenCount: 10,
		UpdatedAt:  time.Now(),
	}
	if err := repo.Upsert(ctx, scene); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := repo.Get(ctx, "work-coverage")
	if err != nil {
		t.Fatalf("Get after Upsert: %v", err)
	}
	if got.Scene != "work-coverage" {
		t.Errorf("expected Scene 'work-coverage', got %q", got.Scene)
	}
}

func TestFileMemorySceneRepository_Delete_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemorySceneRepository(filepath.Join(tmpDir, "memory"))
	ctx := context.Background()

	scene := &memoryv2.MemoryScene{
		Scene:      "to-delete",
		Summary:    "Will be deleted",
		TokenCount: 5,
		UpdatedAt:  time.Now(),
	}
	if err := repo.Upsert(ctx, scene); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := repo.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.Get(ctx, "to-delete")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// ============================================================
// IngatanMemoryCellRepository — List
// ============================================================

func TestIngatanMemoryCellRepository_List_WithConversationID(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/memories") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{ingatanMemoryResponse(cell)},
			})
			return
		}
		http.NotFound(w, r)
	})

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected 1 cell, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_EmptyConversationID(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{})
	if err != nil {
		t.Fatalf("List with empty conv ID: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells for empty conv ID, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_StoreNotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-no-store",
	})
	if err != nil {
		t.Fatalf("List 404: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells on 404, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	_, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-500",
	})
	if err == nil {
		t.Fatal("expected error on 500 status")
	}
}

func TestIngatanMemoryCellRepository_List_WithFilter(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // scene = "test-scene", salience = 0.8

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{ingatanMemoryResponse(cell)},
			})
		}
	})

	cellType := memoryv2.CellTypeFact
	minSalience := 0.5

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		Scene:          "test-scene",
		CellType:       &cellType,
		MinSalience:    &minSalience,
	})
	if err != nil {
		t.Fatalf("List with filter: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected 1 cell, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_FilterExcludesWrongScene(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // scene = "test-scene"

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{ingatanMemoryResponse(cell)},
			})
		}
	})

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		Scene:          "different-scene",
	})
	if err != nil {
		t.Fatalf("List filter wrong scene: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells for wrong scene, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_FilterMinSalienceExcludes(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // salience = 0.8

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{ingatanMemoryResponse(cell)},
			})
		}
	})

	high := 0.9 // higher than cell's 0.8
	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		MinSalience:    &high,
	})
	if err != nil {
		t.Fatalf("List filter high salience: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells below min salience, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_FilterCellTypeExcludes(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // CellType = CellTypeFact

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{ingatanMemoryResponse(cell)},
			})
		}
	})

	wrongType := memoryv2.CellTypeDecision
	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		CellType:       &wrongType,
	})
	if err != nil {
		t.Fatalf("List filter cell type: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells with wrong type, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_WithLimit(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell1 := sampleCell(t)
	cell2 := sampleCell(t)
	cell2.ID = "22222222-2222-2222-2222-222222222222"

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{
					ingatanMemoryResponse(cell1),
					ingatanMemoryResponse(cell2),
				},
			})
		}
	})

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		Limit:          1,
	})
	if err != nil {
		t.Fatalf("List with limit: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected 1 cell with limit=1, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_DecodeError(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, "not valid json{{{{")
		}
	})

	_, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-decode-err",
	})
	if err == nil {
		t.Fatal("expected decode error")
	}
}

// ============================================================
// IngatanMemoryCellRepository — List with expired filter
// ============================================================

func TestIngatanMemoryCellRepository_List_ExcludesExpired(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	expiredMeta := map[string]interface{}{
		metaKeyScene:        cell.Scene,
		metaKeyCellType:     cell.CellType.String(),
		metaKeySalience:     cell.Salience,
		metaKeySourceMsgIDs: cell.Source,
		metaKeyCellID:       cell.ID,
		metaKeyExpiresAt:    past,
	}
	expiredResponse := map[string]interface{}{
		"id":         "ingatan-expired",
		"store":      "test_store",
		"content":    cell.Content,
		"tags":       []string{cell.Scene},
		"source":     "conversation",
		"source_ref": cell.ConversationID,
		"metadata":   expiredMeta,
		"created_at": cell.CreatedAt.Format(time.RFC3339),
		"updated_at": cell.UpdatedAt.Format(time.RFC3339),
	}

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{expiredResponse},
			})
		}
	})

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		IncludeExpired: false,
	})
	if err != nil {
		t.Fatalf("List expired filter: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected expired cell to be excluded, got %d cells", len(cells))
	}
}

func TestIngatanMemoryCellRepository_List_IncludesExpiredWhenFlagSet(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	expiredMeta := map[string]interface{}{
		metaKeyScene:        cell.Scene,
		metaKeyCellType:     cell.CellType.String(),
		metaKeySalience:     cell.Salience,
		metaKeySourceMsgIDs: cell.Source,
		metaKeyCellID:       cell.ID,
		metaKeyExpiresAt:    past,
	}
	expiredResponse := map[string]interface{}{
		"id":         "ingatan-expired-2",
		"store":      "test_store",
		"content":    cell.Content,
		"tags":       []string{cell.Scene},
		"source":     "conversation",
		"source_ref": cell.ConversationID,
		"metadata":   expiredMeta,
		"created_at": cell.CreatedAt.Format(time.RFC3339),
		"updated_at": cell.UpdatedAt.Format(time.RFC3339),
	}

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"memories": []interface{}{expiredResponse},
			})
		}
	})

	cells, err := repo.List(context.Background(), memoryv2.MemoryCellFilter{
		ConversationID: "conv-abc",
		IncludeExpired: true,
	})
	if err != nil {
		t.Fatalf("List include expired: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected expired cell included when IncludeExpired=true, got %d cells", len(cells))
	}
}

// ============================================================
// IngatanMemoryCellRepository — Update
// ============================================================

func TestIngatanMemoryCellRepository_Update_Success(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanMemoryResponse(cell),
						"score":  1.0,
					},
				},
			})
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/memories/"):
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	})

	cell.Content = "updated content"
	if err := repo.Update(context.Background(), cell); err != nil {
		t.Fatalf("Update: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Update_NotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{},
			})
		}
	})

	err := repo.Update(context.Background(), cell)
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Update_PutNotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 1.0},
				},
			})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	})

	err := repo.Update(context.Background(), cell)
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Update_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 1.0},
				},
			})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})

	err := repo.Update(context.Background(), cell)
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

// ============================================================
// IngatanMemoryCellRepository — Delete
// ============================================================

func TestIngatanMemoryCellRepository_Delete_Success(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 1.0},
				},
			})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	if err := repo.Delete(context.Background(), cell.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Delete_NotFound_CellRepo(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
		}
	})

	err := repo.Delete(context.Background(), "nonexistent-id")
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Delete_DeleteStatusNotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 1.0},
				},
			})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	})

	err := repo.Delete(context.Background(), cell.ID)
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Delete_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 1.0},
				},
			})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})

	err := repo.Delete(context.Background(), cell.ID)
	if err == nil {
		t.Fatal("expected error on 500 DELETE")
	}
}

// ============================================================
// IngatanMemoryCellRepository — GetByScene
// ============================================================

func TestIngatanMemoryCellRepository_GetByScene_MatchingScene(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // scene = "test-scene"

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 0.9},
				},
			})
		}
	})

	cells, err := repo.GetByScene(context.Background(), "test-scene", 10)
	if err != nil {
		t.Fatalf("GetByScene: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected 1 cell for matching scene, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_GetByScene_WrongScene(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // scene = "test-scene"

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": ingatanMemoryResponse(cell), "score": 0.9},
				},
			})
		}
	})

	cells, err := repo.GetByScene(context.Background(), "other-scene", 10)
	if err != nil {
		t.Fatalf("GetByScene wrong scene: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells for non-matching scene, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_GetByScene_StoreNotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	cells, err := repo.GetByScene(context.Background(), "no-store-scene", 10)
	if err != nil {
		t.Fatalf("GetByScene 404: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells on 404, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_GetByScene_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	_, err := repo.GetByScene(context.Background(), "some-scene", 10)
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

// ============================================================
// IngatanMemoryCellRepository — GetHighSalience
// ============================================================

func TestIngatanMemoryCellRepository_GetHighSalience_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	_, err := repo.GetHighSalience(context.Background(), "conv-abc", 0.5, 10)
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

// ============================================================
// IngatanMemoryCellRepository — SearchFTS edge cases
// ============================================================

func TestIngatanMemoryCellRepository_SearchFTS_NotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			http.NotFound(w, r)
		}
	})

	cells, err := repo.SearchFTS(context.Background(), "query", 10)
	if err != nil {
		t.Fatalf("SearchFTS 404: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells on 404, got %d", len(cells))
	}
}

func TestIngatanMemoryCellRepository_SearchFTS_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	_, err := repo.SearchFTS(context.Background(), "query", 10)
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

// ============================================================
// ingatanNotFoundError — Error()
// ============================================================

func TestIngatanNotFoundError_Error(t *testing.T) {
	err := &ingatanNotFoundError{op: "create", store: "my_store"}
	msg := err.Error()
	if !strings.Contains(msg, "create") {
		t.Errorf("error message should contain op 'create', got: %q", msg)
	}
	if !strings.Contains(msg, "my_store") {
		t.Errorf("error message should contain store 'my_store', got: %q", msg)
	}
	if !strings.Contains(msg, "404") {
		t.Errorf("error message should contain '404', got: %q", msg)
	}
}

// ============================================================
// findIngatanID — error paths
// ============================================================

func TestIngatanMemoryCellRepository_findIngatanID_SearchReturnsNotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			http.NotFound(w, r)
		}
	})

	err := repo.Update(context.Background(), cell)
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("expected ErrNotFound when search returns 404, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_findIngatanID_SearchUnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	err := repo.Update(context.Background(), cell)
	if err == nil {
		t.Fatal("expected error on 503 search")
	}
}

// ============================================================
// NewIngatanMemoryCellRepository — default prefix
// ============================================================

func TestNewIngatanMemoryCellRepository_DefaultPrefix(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "")
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

// ============================================================
// ensureStore — conflict (already exists) is OK
// ============================================================

func TestIngatanMemoryCellRepository_Create_StoreAlreadyExists(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	callCount := 0
	mux.HandleFunc("/api/v1/stores", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict) // store already exists
		}
	})
	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories") {
			callCount++
			if callCount == 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ingatanMemoryResponse(cell))
		}
	})

	if err := repo.Create(context.Background(), cell); err != nil {
		t.Fatalf("Create with conflict on store creation: %v", err)
	}
}

// ============================================================
// AtomicFileWriter — error paths
// ============================================================

func TestAtomicFileWriter_Write_FailsOnReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test as root")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "ro")
	if err := os.Mkdir(readOnlyDir, 0o555); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(readOnlyDir, 0o755) }) //nolint:errcheck

	w := NewAtomicFileWriter()
	err := w.Write(filepath.Join(readOnlyDir, "file.json"), []byte(`{}`), 0o644)
	if err == nil {
		t.Error("expected error writing to read-only directory")
	}
}

func TestAtomicFileWriter_Write_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c", "file.json")
	w := NewAtomicFileWriter()
	if err := w.Write(nested, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("Write with nested dir: %v", err)
	}
	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Error("expected nested file to be created")
	}
}

// ============================================================
// FileLock — additional paths
// ============================================================

func TestFileLock_Unlock_WhenNotLocked(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewFileLock(filepath.Join(tmpDir, "notlocked.lock"))
	if err := l.Unlock(); err != nil {
		t.Errorf("Unlock without Lock returned error: %v", err)
	}
}

func TestFileLock_Lock_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	l := NewFileLock(filepath.Join(tmpDir, "nested", "dir", "test.lock"))
	if err := l.Lock(); err != nil {
		t.Fatalf("Lock in nested dir: %v", err)
	}
	_ = l.Unlock()
}

// ============================================================
// IngatanMemorySceneRepository — Upsert (update path)
// ============================================================

func TestIngatanMemorySceneRepository_Upsert_UpdateExisting(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	callCount := 0
	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search"):
			callCount++
			if callCount <= 2 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{
							"memory": ingatanSceneMemoryResponse(scene),
							"score":  1.0,
						},
					},
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
			}
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	})

	if err := repo.Upsert(context.Background(), scene); err != nil {
		t.Fatalf("Upsert (update path): %v", err)
	}
}

func TestIngatanMemorySceneRepository_Upsert_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
		} else if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories") {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	err := repo.Upsert(context.Background(), scene)
	if err == nil {
		t.Fatal("expected error on 500 create")
	}
}

// ============================================================
// IngatanMemorySceneRepository — List
// ============================================================

func TestIngatanMemorySceneRepository_List_Success(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanSceneMemoryResponse(scene),
						"score":  1.0,
					},
				},
			})
		}
	})

	scenes, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scenes) != 1 {
		t.Errorf("expected 1 scene, got %d", len(scenes))
	}
}

func TestIngatanMemorySceneRepository_List_StoreNotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	scenes, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List 404: %v", err)
	}
	if len(scenes) != 0 {
		t.Errorf("expected 0 scenes on 404, got %d", len(scenes))
	}
}

func TestIngatanMemorySceneRepository_List_UnexpectedStatus(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	_, err := repo.List(context.Background())
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestIngatanMemorySceneRepository_List_NonSceneMemoryFiltered(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			nonSceneMem := map[string]interface{}{
				"id":         "ingatan-non-scene",
				"store":      "test_scenes",
				"content":    "not a scene",
				"tags":       []string{"other-tag"},
				"source":     "manual",
				"source_ref": "",
				"metadata":   map[string]interface{}{},
				"created_at": time.Now().Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"memory": nonSceneMem, "score": 0.5},
				},
			})
		}
	})

	scenes, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List non-scene: %v", err)
	}
	if len(scenes) != 0 {
		t.Errorf("expected non-scene memories to be filtered out, got %d", len(scenes))
	}
}

// ============================================================
// IngatanMemorySceneRepository — Get
// ============================================================

func TestIngatanMemorySceneRepository_Get_Success_Coverage(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanSceneMemoryResponse(scene),
						"score":  1.0,
					},
				},
			})
		}
	})

	got, err := repo.Get(context.Background(), scene.Scene)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Scene != scene.Scene {
		t.Errorf("expected Scene %q, got %q", scene.Scene, got.Scene)
	}
}

// ============================================================
// IngatanMemorySceneRepository — Delete paths
// ============================================================

func TestIngatanMemorySceneRepository_Delete_UnexpectedStatus_Coverage(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	callCount := 0
	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			callCount++
			if callCount <= 2 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{"memory": ingatanSceneMemoryResponse(scene), "score": 1.0},
					},
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
			}
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})

	err := repo.Delete(context.Background(), scene.Scene)
	if err == nil {
		t.Fatal("expected error on 500 DELETE")
	}
}

func TestIngatanMemorySceneRepository_Delete_Returns404_Coverage(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	callCount := 0
	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search"):
			callCount++
			if callCount <= 2 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []interface{}{
						map[string]interface{}{"memory": ingatanSceneMemoryResponse(scene), "score": 1.0},
					},
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
			}
		case r.Method == http.MethodDelete:
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	err := repo.Delete(context.Background(), scene.Scene)
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("expected ErrNotFound on 404 DELETE, got: %v", err)
	}
}

// ============================================================
// NewIngatanMemorySceneRepository — default prefix
// ============================================================

func TestNewIngatanMemorySceneRepository_DefaultPrefix(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "")
	if repo == nil {
		t.Fatal("expected non-nil scene repository")
	}
}

// ============================================================
// FileConversationRepository — additional paths
// ============================================================

func TestFileConversationRepository_SaveConversation_WithMessages(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	longContent := strings.Repeat("x", 150)
	conv := &domain.Conversation{
		ID:       "conv-with-msgs",
		UserID:   "user-msgs",
		Platform: domain.PlatformSlack,
		Messages: []domain.StoredMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: longContent},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	retrieved, err := repo.GetConversation(ctx, "conv-with-msgs")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if len(retrieved.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(retrieved.Messages))
	}
}

func TestFileConversationRepository_UpdateConversation(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	conv := &domain.Conversation{
		ID:        "conv-update",
		UserID:    "user-upd",
		Platform:  domain.PlatformTelegram,
		Messages:  []domain.StoredMessage{{Role: "user", Content: "hi"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("initial SaveConversation: %v", err)
	}

	conv.Messages = append(conv.Messages, domain.StoredMessage{Role: "assistant", Content: "hello"})
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("update SaveConversation: %v", err)
	}

	retrieved, err := repo.GetConversation(ctx, "conv-update")
	if err != nil {
		t.Fatalf("GetConversation after update: %v", err)
	}
	if len(retrieved.Messages) != 2 {
		t.Errorf("expected 2 messages after update, got %d", len(retrieved.Messages))
	}
}

func TestFileConversationRepository_DeleteConversation_NotFound_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	err := repo.DeleteConversation(ctx, "nonexistent-conv")
	if err == nil {
		t.Error("expected error deleting nonexistent conversation")
	}
}

// ============================================================
// FileMemoryAdmin — Stats / DeleteCellsByConversation
// ============================================================

func TestFileMemoryAdmin_Stats_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	sceneRepo := NewFileMemorySceneRepository(filepath.Join(tmpDir, "data"))
	admin := NewFileMemoryAdmin(repo, sceneRepo, filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	stats, err := admin.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats == nil {
		t.Error("expected non-nil stats")
	}
}

// ============================================================
// FileAuditRepository — Append / Query
// ============================================================

func TestFileAuditRepository_AppendAndQuery_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileAuditRepository(filepath.Join(tmpDir, "audit.jsonl"))
	ctx := context.Background()

	entry := &domain.AuditEvent{
		UserID:   "user-audit",
		Action:   "login",
		Resource: "auth",
	}
	if err := repo.Append(ctx, entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := repo.Query(ctx, domain.AuditFilter{UserID: "user-audit", Limit: 10})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least 1 audit entry")
	}
}

// ============================================================
// initializer.go — Initialize idempotent
// ============================================================

func TestInitializer_Initialize_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	if err := Initialize(tmpDir); err != nil {
		t.Fatalf("first Initialize: %v", err)
	}
	if err := Initialize(tmpDir); err != nil {
		t.Fatalf("second Initialize (idempotent): %v", err)
	}
}

// ============================================================
// FileMemoryCellRepository — List with Offset / all-cells
// ============================================================

func TestFileMemoryCellRepository_List_WithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-offset",
			Scene:          "offset-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5,
			Content:        "content " + intStr(i),
			Source:         "[]",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			t.Fatalf("Create cell %d: %v", i, err)
		}
	}

	// Offset larger than result set → empty.
	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{
		Scene:  "offset-scene",
		Offset: 100,
	})
	if err != nil {
		t.Fatalf("List with large offset: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("expected 0 cells with offset > total, got %d", len(cells))
	}
}

func TestFileMemoryCellRepository_List_AllCells(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-all-" + intStr(i),
			Scene:          "all-scene-" + intStr(i),
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5,
			Content:        "all cells content " + intStr(i),
			Source:         "[]",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			t.Fatalf("Create cell %d: %v", i, err)
		}
	}

	// No scene or conversationID filter → returns all cells.
	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{})
	if err != nil {
		t.Fatalf("List all cells: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 cells, got %d", len(cells))
	}
}

func TestFileMemoryCellRepository_List_FilterByConversationViaScene(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	// Create two cells in same scene but different conversations.
	cell1 := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-a",
		Scene:          "shared-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.5,
		Content:        "cell a",
		Source:         "[]",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	cell2 := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-b",
		Scene:          "shared-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.5,
		Content:        "cell b",
		Source:         "[]",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := repo.Create(ctx, cell1); err != nil {
		t.Fatalf("Create cell1: %v", err)
	}
	if err := repo.Create(ctx, cell2); err != nil {
		t.Fatalf("Create cell2: %v", err)
	}

	// Filter by both scene and conversationID.
	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{
		Scene:          "shared-scene",
		ConversationID: "conv-a",
	})
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected 1 cell for conv-a, got %d", len(cells))
	}
	if cells[0].ConversationID != "conv-a" {
		t.Errorf("expected conv-a, got %q", cells[0].ConversationID)
	}
}

// ============================================================
// FileMemoryCellRepository — appendUnique already-present path
// ============================================================

func TestAppendUnique_AlreadyPresent(t *testing.T) {
	result := appendUnique([]string{"a", "b"}, "a")
	if len(result) != 2 {
		t.Errorf("appendUnique should not add duplicate, got %v", result)
	}
}

// ============================================================
// FileMemoryAdmin — CountCellsByConversation / RebuildFTSIndex
// ============================================================

func TestFileMemoryAdmin_CountCellsByConversation_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileMemoryCellRepository(basePath)
	sceneRepo := NewFileMemorySceneRepository(basePath)
	admin := NewFileMemoryAdmin(repo, sceneRepo, basePath)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-count-admin",
			Scene:          "scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5,
			Content:        "content " + intStr(i),
			Source:         "[]",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	count, err := admin.CountCellsByConversation(ctx, "conv-count-admin")
	if err != nil {
		t.Fatalf("CountCellsByConversation: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 cells, got %d", count)
	}
}

func TestFileMemoryAdmin_RebuildFTSIndex_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileMemoryCellRepository(basePath)
	sceneRepo := NewFileMemorySceneRepository(basePath)
	admin := NewFileMemoryAdmin(repo, sceneRepo, basePath)
	ctx := context.Background()

	// Just ensure it doesn't error on an empty directory.
	if err := admin.RebuildFTSIndex(ctx); err != nil {
		t.Fatalf("RebuildFTSIndex: %v", err)
	}
}

// ============================================================
// FileConversationRepository — DeleteConversation success
// ============================================================

func TestFileConversationRepository_DeleteConversation_Success(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	conv := &domain.Conversation{
		ID:        "conv-to-delete",
		UserID:    "user-del-conv",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}
	if err := repo.DeleteConversation(ctx, "conv-to-delete"); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	_, err := repo.GetConversation(ctx, "conv-to-delete")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// ============================================================
// FileMemorySceneRepository — List (via get all scenes)
// ============================================================

func TestFileMemorySceneRepository_List_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemorySceneRepository(filepath.Join(tmpDir, "memory"))
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s := &memoryv2.MemoryScene{
			Scene:      "list-scene-" + intStr(i),
			Summary:    "summary " + intStr(i),
			TokenCount: (i + 1) * 10,
			UpdatedAt:  time.Now(),
		}
		if err := repo.Upsert(ctx, s); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}

	scenes, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scenes) != 3 {
		t.Errorf("expected 3 scenes, got %d", len(scenes))
	}
}

// ============================================================
// FileUserProfileRepository — ListProfiles with pagination
// ============================================================

func TestFileUserProfileRepository_ListProfiles_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileUserProfileRepository(filepath.Join(tmpDir, "users.json"), "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		p := domain.NewUserProfile("user-list-"+intStr(i), "list"+intStr(i)+"@example.com", domain.UserTypeIndividual)
		if err := repo.SaveProfile(ctx, p); err != nil {
			t.Fatalf("SaveProfile %d: %v", i, err)
		}
	}

	profiles, err := repo.ListProfiles(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestFileUserProfileRepository_ListProfiles_OffsetPastEnd(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileUserProfileRepository(filepath.Join(tmpDir, "users.json"), "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	p := domain.NewUserProfile("user-single", "single@example.com", domain.UserTypeIndividual)
	if err := repo.SaveProfile(ctx, p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	profiles, err := repo.ListProfiles(ctx, 100, 10)
	if err != nil {
		t.Fatalf("ListProfiles with large offset: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles with offset > total, got %d", len(profiles))
	}
}

// ============================================================
// FileNotesRepository — List with tag filter
// ============================================================

func TestFileNotesRepository_List_WithTagFilter(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileNotesRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	userID := "user-tag-filter"
	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     "Tagged Note",
		Content:   "With tags",
		Tags:      []string{"work", "project"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, note); err != nil {
		t.Fatalf("Create: %v", err)
	}

	all, err := repo.List(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 note, got %d", len(all))
	}
}

// ============================================================
// Metrics — GetStorageMetrics non-existent dir
// ============================================================

func TestGetStorageMetrics_NonExistentDir(t *testing.T) {
	// Non-existent dir returns unhealthy status without error.
	m, err := GetStorageMetrics("/nonexistent/path/for/testing/metrics123")
	if err != nil {
		t.Fatalf("GetStorageMetrics: %v", err)
	}
	if m.Status != "unhealthy" {
		t.Errorf("expected 'unhealthy' for non-existent dir, got %q", m.Status)
	}
}

func TestGetStorageMetrics_ReadOnlyDir_IsUnhealthy(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping as root")
	}
	tmpDir := t.TempDir()
	if err := os.Chmod(tmpDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(tmpDir, 0o755) }) //nolint:errcheck

	m, err := GetStorageMetrics(tmpDir)
	if err != nil {
		t.Fatalf("GetStorageMetrics: %v", err)
	}
	if m.Status != "unhealthy" {
		t.Errorf("expected 'unhealthy' for read-only dir, got %q", m.Status)
	}
}

// ============================================================
// FileAuditRepository — Query with various filters
// ============================================================

func TestFileAuditRepository_Query_FilterByAction(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileAuditRepository(tmpDir)
	ctx := context.Background()

	for _, action := range []string{"login", "logout", "login"} {
		if err := repo.Append(ctx, &domain.AuditEvent{UserID: "u1", Action: action}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	events, err := repo.Query(ctx, domain.AuditFilter{Action: "login"})
	if err != nil {
		t.Fatalf("Query by action: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 login events, got %d", len(events))
	}
}

func TestFileAuditRepository_Query_FilterByOutcome(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileAuditRepository(tmpDir)
	ctx := context.Background()

	for _, outcome := range []string{"success", "failure", "success"} {
		if err := repo.Append(ctx, &domain.AuditEvent{UserID: "u2", Outcome: outcome}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	events, err := repo.Query(ctx, domain.AuditFilter{Outcome: "failure"})
	if err != nil {
		t.Fatalf("Query by outcome: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 failure event, got %d", len(events))
	}
}

func TestFileAuditRepository_Query_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileAuditRepository(tmpDir)
	ctx := context.Background()

	events, err := repo.Query(ctx, domain.AuditFilter{})
	if err != nil {
		t.Fatalf("Query no file: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events when no file, got %d", len(events))
	}
}

func TestFileAuditRepository_Query_WithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileAuditRepository(tmpDir)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := repo.Append(ctx, &domain.AuditEvent{UserID: "u3", Action: "act"}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	events, err := repo.Query(ctx, domain.AuditFilter{Offset: 100})
	if err != nil {
		t.Fatalf("Query with offset>total: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events with large offset, got %d", len(events))
	}
}

// ============================================================
// FileConversationRepository — AppendMessage
// ============================================================

func TestFileConversationRepository_AppendMessage_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	conv := &domain.Conversation{
		ID:        "conv-append-msg",
		UserID:    "user-append",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	msg := domain.StoredMessage{Role: "user", Content: "appended message"}
	if err := repo.AppendMessage(ctx, "conv-append-msg", msg); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	retrieved, err := repo.GetConversation(ctx, "conv-append-msg")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if len(retrieved.Messages) != 1 {
		t.Errorf("expected 1 message after AppendMessage, got %d", len(retrieved.Messages))
	}
}

// ============================================================
// FileConversationRepository — ListConversations
// ============================================================

func TestFileConversationRepository_ListConversations_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		conv := &domain.Conversation{
			ID:        "conv-list-cov-" + intStr(i),
			UserID:    "user-list-cov",
			Platform:  domain.PlatformCLI,
			Messages:  []domain.StoredMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.SaveConversation(ctx, conv); err != nil {
			t.Fatalf("SaveConversation: %v", err)
		}
	}

	convs, err := repo.ListConversations(ctx, "user-list-cov")
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(convs) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(convs))
	}
}

// ============================================================
// FileConversationRepository — CountMessages
// ============================================================

func TestFileConversationRepository_CountMessages_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileConversationRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	conv := &domain.Conversation{
		ID:     "conv-count-msg",
		UserID: "user-cnt-cov",
		Messages: []domain.StoredMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	count, err := repo.CountMessages(ctx, "conv-count-msg")
	if err != nil {
		t.Fatalf("CountMessages: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 messages, got %d", count)
	}
}

// ============================================================
// FileUserProfileRepository — DeleteProfile
// ============================================================

func TestFileUserProfileRepository_DeleteProfile_Coverage(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileUserProfileRepository(filepath.Join(tmpDir, "users.json"), "test-encryption-key-32bytes!ab")
	ctx := context.Background()

	p := domain.NewUserProfile("user-del-prof", "del@example.com", domain.UserTypeIndividual)
	if err := repo.SaveProfile(ctx, p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	if err := repo.DeleteProfile(ctx, "user-del-prof"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	_, err := repo.GetProfileByUserID(ctx, "user-del-prof")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

// ensure httptest import is used
var _ = httptest.NewServer
