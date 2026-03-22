package skill

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// Tests for VersionManager
func TestVersionManager_NewVersionManager(t *testing.T) {
	t.Parallel()
	vm := NewVersionManager()
	if vm == nil {
		t.Fatal("NewVersionManager returned nil")
	}
}

func TestVersionManager_RegisterAndGetLatest(t *testing.T) {
	t.Parallel()

	vm := NewVersionManager()

	v1, _ := domain.ParseVersion("1.0.0")
	v2, _ := domain.ParseVersion("2.0.0")
	v1beta, _ := domain.ParseVersion("1.5.0")

	vm.RegisterVersion("test-skill", v1)
	vm.RegisterVersion("test-skill", v1beta)
	vm.RegisterVersion("test-skill", v2)

	t.Run("returns_latest_version", func(t *testing.T) {
		t.Parallel()
		latest, err := vm.GetLatest("test-skill")
		if err != nil {
			t.Fatalf("GetLatest failed: %v", err)
		}
		if latest.Major != 2 {
			t.Errorf("Expected major version 2, got %d", latest.Major)
		}
	})

	t.Run("returns_all_versions", func(t *testing.T) {
		t.Parallel()
		versions := vm.GetVersions("test-skill")
		if len(versions) != 3 {
			t.Errorf("Expected 3 versions, got %d", len(versions))
		}
	})
}

func TestVersionManager_GetLatest_NoVersions(t *testing.T) {
	t.Parallel()

	vm := NewVersionManager()
	_, err := vm.GetLatest("nonexistent-skill")
	if err == nil {
		t.Error("Expected error for nonexistent skill")
	}
}

func TestVersionManager_GetVersions_Empty(t *testing.T) {
	t.Parallel()

	vm := NewVersionManager()
	versions := vm.GetVersions("nonexistent-skill")
	if versions != nil {
		t.Errorf("Expected nil for nonexistent skill, got %v", versions)
	}
}

// Tests for MemoryAPI
type mockMemoryQuery struct {
	memories map[string]*domain.SkillMemory
	setErr   error
	getErr   error
	delErr   error
	listErr  error
}

func newMockMemoryQuery() *mockMemoryQuery {
	return &mockMemoryQuery{
		memories: make(map[string]*domain.SkillMemory),
	}
}

func (m *mockMemoryQuery) Set(memory *domain.SkillMemory) error {
	if m.setErr != nil {
		return m.setErr
	}
	key := memory.SkillName + ":" + memory.Key + ":" + string(memory.Scope)
	m.memories[key] = memory
	return nil
}

func (m *mockMemoryQuery) Get(skillName, key string, scope domain.MemoryScope) (*domain.SkillMemory, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	k := skillName + ":" + key + ":" + string(scope)
	mem, ok := m.memories[k]
	if !ok {
		return nil, errors.New("not found")
	}
	return mem, nil
}

func (m *mockMemoryQuery) Delete(skillName, key string, scope domain.MemoryScope) error {
	if m.delErr != nil {
		return m.delErr
	}
	k := skillName + ":" + key + ":" + string(scope)
	delete(m.memories, k)
	return nil
}

func (m *mockMemoryQuery) List(skillName string, scope domain.MemoryScope) ([]*domain.SkillMemory, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*domain.SkillMemory
	for _, mem := range m.memories {
		if mem.SkillName == skillName && mem.Scope == scope {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *mockMemoryQuery) Cleanup() error {
	return nil
}

func TestMemoryAPI_NewMemoryAPI(t *testing.T) {
	t.Parallel()
	storage := newMockMemoryQuery()
	api := NewMemoryAPI(storage)
	if api == nil {
		t.Fatal("NewMemoryAPI returned nil")
	}
}

func TestMemoryAPI_Remember(t *testing.T) {
	t.Parallel()

	t.Run("stores_value", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		api := NewMemoryAPI(storage)

		err := api.Remember("test-skill", "key1", "value1", domain.MemoryScopeSkill)
		if err != nil {
			t.Fatalf("Remember failed: %v", err)
		}
	})

	t.Run("storage_error_propagated", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		storage.setErr = errors.New("storage error")
		api := NewMemoryAPI(storage)

		err := api.Remember("test-skill", "key1", "value1", domain.MemoryScopeSkill)
		if err == nil {
			t.Error("Expected error when storage fails")
		}
	})
}

func TestMemoryAPI_Recall(t *testing.T) {
	t.Parallel()

	t.Run("retrieves_value", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		api := NewMemoryAPI(storage)

		// Store a value first
		err := api.Remember("test-skill", "key1", "hello world", domain.MemoryScopeSkill)
		if err != nil {
			t.Fatalf("Remember failed: %v", err)
		}

		var dest string
		err = api.Recall("test-skill", "key1", domain.MemoryScopeSkill, &dest)
		if err != nil {
			t.Fatalf("Recall failed: %v", err)
		}
		if dest != "hello world" {
			t.Errorf("Expected 'hello world', got %q", dest)
		}
	})

	t.Run("not_found_error", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		api := NewMemoryAPI(storage)

		var dest string
		err := api.Recall("test-skill", "nonexistent", domain.MemoryScopeSkill, &dest)
		if err == nil {
			t.Error("Expected error for nonexistent key")
		}
	})

	t.Run("expired_memory_returns_error", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		api := NewMemoryAPI(storage)

		// Insert an already-expired memory
		past := time.Now().Add(-1 * time.Hour)
		storage.memories["test-skill:expkey:skill"] = &domain.SkillMemory{
			SkillName: "test-skill",
			Key:       "expkey",
			Value:     `"value"`,
			Scope:     domain.MemoryScopeSkill,
			ExpiresAt: &past,
		}

		var dest string
		err := api.Recall("test-skill", "expkey", domain.MemoryScopeSkill, &dest)
		if err == nil {
			t.Error("Expected error for expired memory")
		}
	})
}

func TestMemoryAPI_Forget(t *testing.T) {
	t.Parallel()

	storage := newMockMemoryQuery()
	api := NewMemoryAPI(storage)

	err := api.Remember("test-skill", "key1", "value1", domain.MemoryScopeSkill)
	if err != nil {
		t.Fatalf("Remember failed: %v", err)
	}

	err = api.Forget("test-skill", "key1", domain.MemoryScopeSkill)
	if err != nil {
		t.Fatalf("Forget failed: %v", err)
	}

	// Should no longer be retrievable
	var dest string
	err = api.Recall("test-skill", "key1", domain.MemoryScopeSkill, &dest)
	if err == nil {
		t.Error("Expected error after forgetting key")
	}
}

func TestMemoryAPI_ListMemories(t *testing.T) {
	t.Parallel()

	t.Run("returns_valid_memories", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		api := NewMemoryAPI(storage)

		api.Remember("test-skill", "k1", "v1", domain.MemoryScopeSkill) //nolint:errcheck
		api.Remember("test-skill", "k2", "v2", domain.MemoryScopeSkill) //nolint:errcheck

		memories, err := api.ListMemories("test-skill", domain.MemoryScopeSkill)
		if err != nil {
			t.Fatalf("ListMemories failed: %v", err)
		}
		if len(memories) != 2 {
			t.Errorf("Expected 2 memories, got %d", len(memories))
		}
	})

	t.Run("filters_expired_memories", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		api := NewMemoryAPI(storage)

		// One valid, one expired
		api.Remember("test-skill", "valid", "v1", domain.MemoryScopeSkill) //nolint:errcheck

		past := time.Now().Add(-1 * time.Hour)
		storage.memories["test-skill:expired:skill"] = &domain.SkillMemory{
			SkillName: "test-skill",
			Key:       "expired",
			Value:     `"value"`,
			Scope:     domain.MemoryScopeSkill,
			ExpiresAt: &past,
		}

		memories, err := api.ListMemories("test-skill", domain.MemoryScopeSkill)
		if err != nil {
			t.Fatalf("ListMemories failed: %v", err)
		}
		if len(memories) != 1 {
			t.Errorf("Expected 1 valid memory after filtering expired, got %d", len(memories))
		}
	})

	t.Run("list_error_propagated", func(t *testing.T) {
		t.Parallel()
		storage := newMockMemoryQuery()
		storage.listErr = errors.New("list error")
		api := NewMemoryAPI(storage)

		_, err := api.ListMemories("test-skill", domain.MemoryScopeSkill)
		if err == nil {
			t.Error("Expected error when list fails")
		}
	})
}

// Tests for registry Initialize
func TestInMemorySkillRegistry_Initialize(t *testing.T) {
	t.Parallel()

	t.Run("successful_scan", func(t *testing.T) {
		t.Parallel()
		skills := []domain.Skill{
			{
				Name:        "skill-1",
				Description: "Skill 1",
				Scope:       domain.ScopeProject,
				Priority:    100,
				Frontmatter: domain.SkillFrontmatter{
					Name:        "skill-1",
					Description: "Skill 1",
				},
			},
		}
		repo := &mockSkillRepository{skills: skills}
		registry := NewInMemorySkillRegistry(repo)

		roots := []domain.SkillRoot{{Path: "/test/path", Scope: domain.ScopeProject}}
		err := registry.Initialize(context.Background(), roots)
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Verify skill was registered
		_, err = registry.Get("skill-1")
		if err != nil {
			t.Errorf("Expected skill to be registered after Initialize: %v", err)
		}
	})

	t.Run("scan_error_propagated", func(t *testing.T) {
		t.Parallel()
		repo := &mockSkillRepository{err: errors.New("scan failed")}
		registry := NewInMemorySkillRegistry(repo)

		roots := []domain.SkillRoot{{Path: "/test/path", Scope: domain.ScopeProject}}
		err := registry.Initialize(context.Background(), roots)
		if err == nil {
			t.Error("Expected error when scan fails")
		}
	})
}

// Test for executeCommand error path in preprocess_renderer
func TestPreprocessRenderer_ExecuteCommandError(t *testing.T) {
	t.Parallel()

	executor := &mockErrorCommandExecutor{err: errors.New("execution failed")}
	renderer := NewPreprocessRenderer(executor)

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `Before command

!command
some-command

After command`,
	}

	result, err := renderer.Render(context.Background(), skill, []string{})
	// Should not return an error, but the output should contain an error message
	if err != nil {
		t.Fatalf("Render should not fail on command error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

// mockErrorCommandExecutor always returns an error
type mockErrorCommandExecutor struct {
	err error
}

func (m *mockErrorCommandExecutor) Execute(ctx context.Context, cmd domain.PreprocessCommand) (*domain.CommandResult, error) {
	return nil, m.err
}

// Test for command at end of file (no trailing newline)
func TestPreprocessRenderer_CommandAtEndOfFile(t *testing.T) {
	t.Parallel()

	renderer := NewPreprocessRenderer(NewMockCommandExecutor())

	// Command block without trailing empty line
	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `Intro
!command
ls -la`,
	}

	result, err := renderer.Render(context.Background(), skill, []string{})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

// Test for Render error (argRenderer failure) - currently argRenderer doesn't error in basic cases
// but we test it to be thorough
func TestPreprocessRenderer_RenderNoError(t *testing.T) {
	t.Parallel()

	renderer := NewPreprocessRenderer(NewMockCommandExecutor())

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test with args",
		Frontmatter: domain.SkillFrontmatter{
			Name:        "test-skill",
			Description: "Test",
		},
		BodyMD: "No commands, just content",
	}

	result, err := renderer.Render(context.Background(), skill, []string{"arg1"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}
