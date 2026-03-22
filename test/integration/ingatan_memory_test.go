//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
)

// newIngatanCellRepo creates an IngatanMemoryCellRepository connected to the real Ingatan server.
func newIngatanCellRepo(t *testing.T) *storage.IngatanMemoryCellRepository {
	t.Helper()
	cfg := storage.IngatanClientConfig{
		BaseURL:     ingatanURL(),
		APIKey:      "test-api-key",
		StorePrefix: "inttest",
	}
	client := storage.NewIngatanHTTPClient(cfg)
	return storage.NewIngatanMemoryCellRepository(client, "inttest")
}

// newTestCell creates a valid MemoryCell for testing.
func newTestCell(conversationID, scene, content string) *memoryv2.MemoryCell {
	now := time.Now().UTC().Truncate(time.Second)
	return &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Scene:          scene,
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        content,
		Source:         `["msg-001"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// TestIngatanHybridSearch tests that SearchFTS returns results for a seeded cell.
func TestIngatanHybridSearch(t *testing.T) {
	skipIfNoIngatan(t)

	repo := newIngatanCellRepo(t)
	ctx := context.Background()
	convID := "inttest-hybrid-" + uuid.New().String()

	// Seed a cell.
	cell := newTestCell(convID, "test-scene", "The quick brown fox jumps over the lazy dog")
	require.NoError(t, repo.Create(ctx, cell))
	t.Cleanup(func() {
		_ = repo.Delete(context.Background(), cell.ID)
	})

	// Allow Ingatan time to index.
	time.Sleep(500 * time.Millisecond)

	// Search using a partial keyword.
	results, err := repo.SearchFTS(ctx, "quick brown fox", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results, "expected at least one result for seeded cell")

	// Verify the returned cell matches the seeded content.
	found := false
	for _, r := range results {
		if r.ID == cell.ID {
			found = true
			assert.Equal(t, cell.Content, r.Content)
			break
		}
	}
	assert.True(t, found, "seeded cell should appear in search results")
}

// TestIngatanCrossUserIsolation verifies that user A's cells are not returned
// when searching in user B's store.
func TestIngatanCrossUserIsolation(t *testing.T) {
	skipIfNoIngatan(t)

	repoA := storage.NewIngatanMemoryCellRepository(
		storage.NewIngatanHTTPClient(storage.IngatanClientConfig{
			BaseURL: ingatanURL(), APIKey: "test-api-key", StorePrefix: "inttest",
		}),
		"inttest",
	)
	repoB := storage.NewIngatanMemoryCellRepository(
		storage.NewIngatanHTTPClient(storage.IngatanClientConfig{
			BaseURL: ingatanURL(), APIKey: "test-api-key", StorePrefix: "inttest",
		}),
		"inttest",
	)
	ctx := context.Background()

	userA := "inttest-user-a-" + uuid.New().String()

	cellA := newTestCell(userA, "secret-scene", "This is private information for user A only")
	require.NoError(t, repoA.Create(ctx, cellA))
	t.Cleanup(func() {
		_ = repoA.Delete(context.Background(), cellA.ID)
	})

	// Allow Ingatan time to index.
	time.Sleep(500 * time.Millisecond)

	// User B searching should not find user A's data (different stores).
	results, err := repoB.SearchFTS(ctx, "private information user A", 10)
	require.NoError(t, err)

	for _, r := range results {
		assert.NotEqual(t, cellA.ID, r.ID, "user B should not see user A's cells")
	}
}

// TestIngatanDeleteExpiredIsNoOp verifies that DeleteExpired returns (0, nil).
// Per ADR-4, the Ingatan adapter does not implement TTL deletion.
func TestIngatanDeleteExpiredIsNoOp(t *testing.T) {
	skipIfNoIngatan(t)

	repo := newIngatanCellRepo(t)
	ctx := context.Background()

	count, err := repo.DeleteExpired(ctx)
	require.NoError(t, err, "DeleteExpired should not return an error")
	assert.Equal(t, 0, count, "DeleteExpired should return 0 (no-op per ADR-4)")
}

// TestIngatanSceneRoundTrip tests Upsert and Get for MemoryScenes via Ingatan.
func TestIngatanSceneRoundTrip(t *testing.T) {
	skipIfNoIngatan(t)

	client := storage.NewIngatanHTTPClient(storage.IngatanClientConfig{
		BaseURL:     ingatanURL(),
		APIKey:      "test-api-key",
		StorePrefix: "inttest",
	})
	sceneRepo := storage.NewIngatanMemorySceneRepository(client, "inttest")

	ctx := context.Background()
	sceneName := "inttest-" + uuid.New().String()[:8]

	scene := &memoryv2.MemoryScene{
		Scene:      sceneName,
		Summary:    "This is a test scene summary created during integration testing.",
		TokenCount: 50,
		UpdatedAt:  time.Now().UTC().Truncate(time.Second),
	}

	t.Cleanup(func() {
		_ = sceneRepo.Delete(context.Background(), scene.Scene)
	})

	require.NoError(t, sceneRepo.Upsert(ctx, scene))

	got, err := sceneRepo.Get(ctx, scene.Scene)
	require.NoError(t, err)
	assert.Equal(t, scene.Scene, got.Scene)
	assert.Equal(t, scene.Summary, got.Summary)
	assert.Equal(t, scene.TokenCount, got.TokenCount)
}
