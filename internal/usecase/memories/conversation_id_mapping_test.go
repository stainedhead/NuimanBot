package memories

import (
	"context"
	"encoding/json"
	"testing"

	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
	memoryv2usecase "nuimanbot/internal/usecase/memoryv2"
)

// fakeCuratorLLM is a memoryv2usecase.LLMClient test double that returns a
// single fixed extracted cell, so the real MemoryCuratorService.ExtractCells
// path can be exercised deterministically (no live LLM call).
type fakeCuratorLLM struct {
	scene   string
	content string
}

func (f *fakeCuratorLLM) GenerateJSON(_ context.Context, _, _ string, _ interface{}) (string, error) {
	resp := memoryv2usecase.ExtractionResponse{
		Cells: []memoryv2usecase.ExtractedCell{
			{
				Scene:    f.scene,
				CellType: "fact",
				Salience: 0.8,
				Content:  f.content,
				Source:   []string{"msg-1"},
			},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// TestFR_R7_ConversationIDMapping_TracesRealCLIGatewayFormat is FR-R7's
// required integration test: it creates a memory cell via the REAL curator
// path (internal/usecase/memoryv2.MemoryCuratorService.ExtractCells, backed
// by a real file-based MemoryCellRepository — not a synthetic
// ConversationID = username fixture), using the exact ConversationID format
// the production CLI gateway actually produces, then asserts what
// memories.Service.ListCells currently returns for the web-admin owner a
// human operator would naturally use.
//
// Traced end-to-end from source, cited here for anyone re-verifying this:
//   - internal/usecase/chat/service.go's getConversationID(platform,
//     platformUID) returns platform + ":" + platformUID — a stable
//     composite key, but NOT the web-admin session Username used everywhere
//     else in this codebase (Jobs/Chores/Projects/History/Chats) as
//     ownerUserID.
//   - internal/adapter/gateway/cli/gateway.go's Run loop hardcodes
//     PlatformUID: "cli_user" (a literal placeholder, not tied to any real
//     per-operator identity) for every CLI message, so today EVERY CLI
//     interaction's ConversationID is the single literal string
//     "cli:cli_user", regardless of who is actually typing.
//   - internal/adapter/web/auth.go's login/session handling never
//     references domain.UserRepository/domain.User at all — the web admin
//     account system and the domain.User/gateway-identity system
//     (domain.User.PlatformIDs) are two entirely separate, unbridged
//     identity spaces in this codebase today.
//
// This test proves conversationIDFor(ownerUserID) = ownerUserID (the
// current mapping) can never match a cell created via the real CLI curator
// path unless the web-admin username happens to be the literal string
// "cli:cli_user" — i.e. the finding is a real, confirmed gap, not merely an
// unverified assumption. See implementation-notes.md's Deviations from Plan
// for why the full fix (a real identity bridge) is out of scope for this
// pass, matching FR-R1's precedent.
func TestFR_R7_ConversationIDMapping_TracesRealCLIGatewayFormat(t *testing.T) {
	dir := t.TempDir()
	cellRepo := storage.NewFileMemoryCellRepository(dir)
	sceneRepo := storage.NewFileMemorySceneRepository(dir)

	curator := memoryv2usecase.NewMemoryCuratorService(
		&fakeCuratorLLM{scene: "trip-planning", content: "The operator prefers window seats"},
		cellRepo,
		sceneRepo,
		memoryv2usecase.CuratorConfig{Enabled: true, MaxCellsPerExtraction: 10},
	)

	// The exact literal format the real CLI gateway produces today (see the
	// doc comment above for the citation trail) — not a synthetic fixture.
	const realCLIConversationID = "cli:cli_user"

	result, err := curator.ExtractCells(context.Background(), memoryv2usecase.InteractionContext{
		ConversationID: realCLIConversationID,
		UserMessage:    "I like window seats when I fly",
		AssistantReply: "Noted, I'll remember that for future trip planning.",
	})
	if err != nil {
		t.Fatalf("real curator path failed: %v", err)
	}
	if result.CellsCreated != 1 {
		t.Fatalf("expected the real curator path to create 1 cell, got %d (errors: %v)", result.CellsCreated, result.Errors)
	}

	svc := NewService(cellRepo)

	// A human operator's actual web-admin session username, as it would
	// realistically be set up (never "cli:cli_user" — that's the internal
	// gateway-format string, not a username a person would choose or be
	// assigned as their web login).
	const webAdminOwnerUserID = "admin"

	cells, err := svc.ListCells(context.Background(), webAdminOwnerUserID, memoryv2.MemoryCellFilter{})
	if err != nil {
		t.Fatalf("ListCells failed: %v", err)
	}
	if len(cells) != 0 {
		t.Fatalf("BUG NOW FIXED? re-check this test: expected the current ownerUserID-as-ConversationID "+
			"pass-through to under-show this real CLI-gateway cell (confirming FR-R7's finding), but got %d cells back — "+
			"if a real identity mapping was added, update this test to assert the cell IS now visible instead", len(cells))
	}

	// Conversely: the cell IS visible if a caller queries with the actual
	// ConversationID the gateway used, verbatim — proving the data itself
	// is fine, only the ownerUserID->ConversationID mapping is the gap.
	rawCells, err := svc.cells.List(context.Background(), memoryv2.MemoryCellFilter{ConversationID: realCLIConversationID})
	if err != nil {
		t.Fatalf("direct repository List failed: %v", err)
	}
	if len(rawCells) != 1 {
		t.Fatalf("expected the cell to exist under its real ConversationID, got %d", len(rawCells))
	}
}
