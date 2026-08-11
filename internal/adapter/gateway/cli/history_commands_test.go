package cli_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/history"
)

// newTestHistoryHandler builds a HistoryCommandHandler backed by a real
// history.Service over a temp-dir file repository — mirrors the test
// convention already used in internal/usecase/history/service_test.go rather
// than inventing a separate mock, since the real service is simple and fast
// enough for unit tests.
func newTestHistoryHandler(t *testing.T) (*cli.HistoryCommandHandler, domain.RunRepository) {
	t.Helper()
	repo := storage.NewFileRunRepository(t.TempDir())
	svc := history.NewService(repo)
	return cli.NewHistoryCommandHandler(svc), repo
}

func mustSaveRun(t *testing.T, repo domain.RunRepository, run *domain.Run) {
	t.Helper()
	if err := repo.SaveRun(context.Background(), run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
}

func testUser() *domain.User {
	return &domain.User{ID: "u1", Username: "alice", Role: domain.RoleUser}
}

func TestHistoryCommand_BareShowsHelp(t *testing.T) {
	h, _ := newTestHistoryHandler(t)
	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "/history list") || !strings.Contains(got, "/history show") {
		t.Errorf("expected help text listing subcommands, got:\n%s", got)
	}
}

func TestHistoryCommand_List_NoFilters(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	now := time.Now()
	mustSaveRun(t, repo, &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: now})
	mustSaveRun(t, repo, &domain.Run{ID: "run-2", OwnerUserID: "alice", SourceType: domain.SourceTypeChore, SourceID: "chore-1", Status: domain.RunStatusFailed, CreatedAt: now.Add(-time.Hour)})

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "run-1") || !strings.Contains(got, "run-2") {
		t.Errorf("expected both runs listed, got:\n%s", got)
	}
}

func TestHistoryCommand_List_FilterByJob(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	now := time.Now()
	mustSaveRun(t, repo, &domain.Run{ID: "job-run", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: now})
	mustSaveRun(t, repo, &domain.Run{ID: "chore-run", OwnerUserID: "alice", SourceType: domain.SourceTypeChore, SourceID: "chore-1", Status: domain.RunStatusFailed, CreatedAt: now})

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list --job job-1")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "job-run") {
		t.Errorf("expected job-run in output, got:\n%s", got)
	}
	if strings.Contains(got, "chore-run") {
		t.Errorf("expected chore-run to be filtered out, got:\n%s", got)
	}
}

func TestHistoryCommand_List_FilterByChore(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	now := time.Now()
	mustSaveRun(t, repo, &domain.Run{ID: "job-run", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: now})
	mustSaveRun(t, repo, &domain.Run{ID: "chore-run", OwnerUserID: "alice", SourceType: domain.SourceTypeChore, SourceID: "chore-1", Status: domain.RunStatusFailed, CreatedAt: now})

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list --chore chore-1")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "chore-run") {
		t.Errorf("expected chore-run in output, got:\n%s", got)
	}
	if strings.Contains(got, "job-run") {
		t.Errorf("expected job-run to be filtered out, got:\n%s", got)
	}
}

func TestHistoryCommand_List_FilterByStatus(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	now := time.Now()
	mustSaveRun(t, repo, &domain.Run{ID: "ok-run", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now})
	mustSaveRun(t, repo, &domain.Run{ID: "bad-run", OwnerUserID: "alice", Status: domain.RunStatusFailed, CreatedAt: now})

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list --status failed")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "bad-run") {
		t.Errorf("expected bad-run in output, got:\n%s", got)
	}
	if strings.Contains(got, "ok-run") {
		t.Errorf("expected ok-run to be filtered out, got:\n%s", got)
	}
}

func TestHistoryCommand_List_FilterBySince(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	now := time.Now()
	mustSaveRun(t, repo, &domain.Run{ID: "recent-run", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now})
	mustSaveRun(t, repo, &domain.Run{ID: "old-run", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-48 * time.Hour)})

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list --since 1h")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "recent-run") {
		t.Errorf("expected recent-run in output, got:\n%s", got)
	}
	if strings.Contains(got, "old-run") {
		t.Errorf("expected old-run to be filtered out by --since, got:\n%s", got)
	}
}

func TestHistoryCommand_List_FilterBySince_DateForm(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	mustSaveRun(t, repo, &domain.Run{ID: "future-run", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: time.Now().Add(24 * time.Hour)})
	mustSaveRun(t, repo, &domain.Run{ID: "past-run", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: time.Now().Add(-24 * 365 * time.Hour)})

	since := time.Now().Format("2006-01-02")
	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list --since "+since)
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "future-run") {
		t.Errorf("expected future-run in output, got:\n%s", got)
	}
	if strings.Contains(got, "past-run") {
		t.Errorf("expected past-run filtered out by date --since, got:\n%s", got)
	}
}

func TestHistoryCommand_List_InvalidSince(t *testing.T) {
	h, _ := newTestHistoryHandler(t)
	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list --since not-a-time")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "invalid") {
		t.Errorf("expected an invalid --since message, got:\n%s", got)
	}
}

func TestHistoryCommand_List_Truncates(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	now := time.Now()
	const total = 25
	for i := 0; i < total; i++ {
		mustSaveRun(t, repo, &domain.Run{
			ID:          fmt.Sprintf("run-%02d", i),
			OwnerUserID: "alice",
			Status:      domain.RunStatusCompleted,
			CreatedAt:   now.Add(-time.Duration(i) * time.Minute),
		})
	}

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}

	shown := strings.Count(got, "run-")
	if shown >= total {
		t.Errorf("expected output truncated below %d runs, counted %d mentions:\n%s", total, shown, got)
	}
	if !strings.Contains(got, "more result") {
		t.Errorf("expected a truncation notice mentioning remaining results, got:\n%s", got)
	}
}

func TestHistoryCommand_Show_Found(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	started := time.Now().Add(-time.Hour)
	ended := time.Now()
	mustSaveRun(t, repo, &domain.Run{
		ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1",
		Status: domain.RunStatusCompleted, StartedAt: &started, EndedAt: &ended,
	})

	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history show run-1")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "run-1") || !strings.Contains(got, "job-1") {
		t.Errorf("expected run details in output, got:\n%s", got)
	}
}

func TestHistoryCommand_Show_MarksViewed(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	mustSaveRun(t, repo, &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusCompleted})

	if _, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history show run-1"); err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}

	got, err := repo.GetRun(context.Background(), "alice", "run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.NotifiedAt == nil {
		t.Error("expected showing a run to mark it viewed")
	}
}

func TestHistoryCommand_Show_NotFound(t *testing.T) {
	h, _ := newTestHistoryHandler(t)
	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history show missing-run")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "not found") {
		t.Errorf("expected a not-found message, got:\n%s", got)
	}
}

// TestHistoryCommand_UsesOwnerUserIDNotCurrentUserID verifies AD-5: the
// service is queried using the ownerUserID parameter (the authenticated
// session's Username), never currentUser.ID.
func TestHistoryCommand_UsesOwnerUserIDNotCurrentUserID(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	mustSaveRun(t, repo, &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusCompleted})

	// currentUser.ID deliberately does NOT match ownerUserID, to prove the
	// handler uses the ownerUserID parameter, not currentUser.ID.
	mismatchedUser := &domain.User{ID: "some-other-internal-id", Username: "alice", Role: domain.RoleUser}

	got, err := h.HandleHistoryCommand(context.Background(), mismatchedUser, "alice", "/history show run-1")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(got, "run-1") {
		t.Errorf("expected the run to be found via ownerUserID (session.Username), got:\n%s", got)
	}
}

// TestHistoryCommand_CrossUserIsolation verifies that user A cannot see user
// B's run history via /history list or /history show.
func TestHistoryCommand_CrossUserIsolation(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	mustSaveRun(t, repo, &domain.Run{ID: "bob-run", OwnerUserID: "bob", Status: domain.RunStatusCompleted, CreatedAt: time.Now()})

	listOut, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history list")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if strings.Contains(listOut, "bob-run") {
		t.Errorf("expected alice's /history list to exclude bob's run, got:\n%s", listOut)
	}

	showOut, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history show bob-run")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(strings.ToLower(showOut), "not found") {
		t.Errorf("expected alice's /history show of bob's run to be not-found, got:\n%s", showOut)
	}
}

func TestHistoryCommand_UnknownSubcommand(t *testing.T) {
	h, _ := newTestHistoryHandler(t)
	got, err := h.HandleHistoryCommand(context.Background(), testUser(), "alice", "/history bogus")
	if err != nil {
		t.Fatalf("HandleHistoryCommand: %v", err)
	}
	if !strings.Contains(strings.ToLower(got), "unknown") {
		t.Errorf("expected an unknown-subcommand message, got:\n%s", got)
	}
}

// TestHistoryCommandHandler_SatisfiesEnvCommandHandler verifies
// *HistoryCommandHandler implements cli.EnvCommandHandler directly via
// Handle, and that Handle delegates to HandleHistoryCommand.
func TestHistoryCommandHandler_SatisfiesEnvCommandHandler(t *testing.T) {
	h, repo := newTestHistoryHandler(t)
	mustSaveRun(t, repo, &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusCompleted})

	var envHandler cli.EnvCommandHandler = h
	got, err := envHandler.Handle(context.Background(), testUser(), "alice", "/history show run-1")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(got, "run-1") {
		t.Errorf("expected Handle to delegate to HandleHistoryCommand, got:\n%s", got)
	}
}
