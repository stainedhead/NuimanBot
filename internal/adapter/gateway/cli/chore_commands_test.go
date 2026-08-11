package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/chores"
)

// fakeChoreEvaluator is a table-driven chores.ScheduleEvaluator test double,
// mirroring internal/usecase/chores/service_test.go's own fakeEvaluator
// (duplicated locally since that type is unexported in its package).
type fakeChoreEvaluator struct {
	fixedNext time.Time
}

func newFakeChoreEvaluator() *fakeChoreEvaluator {
	return &fakeChoreEvaluator{fixedNext: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)}
}

func (f *fakeChoreEvaluator) Validate(cronExpr string) error {
	if cronExpr == "bogus cron" {
		return errors.New("invalid cron expression")
	}
	return nil
}

func (f *fakeChoreEvaluator) NextFireTime(_ string, _ time.Time) (time.Time, error) {
	return f.fixedNext, nil
}

// newTestChoreHandler returns a ChoreCommandHandler backed by a real
// chores.Service (file-based repo/store, per internal/usecase/chores's own
// test convention), plus the domain.RunRepository backing DeleteChore's
// active-run check, so tests can seed a Run to exercise the soft-delete
// path.
func newTestChoreHandler(t *testing.T) (*ChoreCommandHandler, domain.RunRepository) {
	t.Helper()
	repo := storage.NewFileChoreRepository(t.TempDir())
	runRepo := storage.NewFileRunRepository(t.TempDir())
	service := chores.NewService(repo, newFakeChoreEvaluator(), runRepo, storage.NewFileConfinedFileStore(), t.TempDir())
	return NewChoreCommandHandler(service), runRepo
}

// extractChoreID pulls the "ID: <uuid>" value out of createChore's
// formatted output.
func extractChoreID(t *testing.T, createOutput string) string {
	t.Helper()
	for _, line := range strings.Split(createOutput, "\n") {
		if strings.HasPrefix(line, "ID: ") {
			return strings.TrimPrefix(line, "ID: ")
		}
	}
	t.Fatalf("no ID line found in create output: %q", createOutput)
	return ""
}

func TestHandleChoreCommand_BareShowsHelp(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Chore Commands") {
		t.Errorf("expected help text, got: %s", result)
	}
}

func TestHandleChoreCommand_ListEmpty(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No chores found") {
		t.Errorf("expected empty-list message, got: %s", result)
	}
}

func TestHandleChoreCommand_CreateAndList(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	createResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore create backup run-the-backup --dir /tmp/work --schedule daily")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !strings.Contains(createResult, "backup") || !strings.Contains(createResult, "daily") {
		t.Errorf("unexpected create output: %s", createResult)
	}
	if !strings.Contains(createResult, "2026-08-12") {
		t.Errorf("expected confirmed next-run time in create output, got: %s", createResult)
	}

	listResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(listResult, "backup") {
		t.Errorf("expected backup in list output, got: %s", listResult)
	}
}

func TestHandleChoreCommand_CreateWithRawCronExpression(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	// A raw cron expression contains spaces; --schedule must consume the
	// remainder of the arguments to support this.
	createResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore create sync d --schedule */5 * * * *")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !strings.Contains(createResult, "*/5 * * * *") {
		t.Errorf("expected raw cron expression in create output, got: %s", createResult)
	}
}

func TestHandleChoreCommand_CreateUsage(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore create backup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Usage:") {
		t.Errorf("expected usage message, got: %s", result)
	}
}

func TestHandleChoreCommand_CreateMissingSchedule(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore create backup d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Usage:") {
		t.Errorf("expected usage message when --schedule is missing, got: %s", result)
	}
}

func TestHandleChoreCommand_CreateInvalidSchedule(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore create backup d --schedule bogus cron")
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression")
	}
}

func TestHandleChoreCommand_ShowFound(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	createResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore create backup run-it --dir /tmp/work --schedule daily")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractChoreID(t, createResult)

	showResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore show "+id)
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if !strings.Contains(showResult, "/tmp/work") {
		t.Errorf("expected working directory in show output, got: %s", showResult)
	}
	if !strings.Contains(showResult, "Confirmed: true") {
		t.Errorf("expected confirmed status, got: %s", showResult)
	}
	if !strings.Contains(showResult, "daily") {
		t.Errorf("expected schedule in show output, got: %s", showResult)
	}
}

func TestHandleChoreCommand_ShowNotFound(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore show does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected not-found message, got: %s", result)
	}
}

// TestHandleChoreCommand_ConfirmScheduleTwoStepFlow proves /chore
// confirm-schedule flips an unconfirmed Chore's ScheduleConfirmed and
// computes its NextFireTime, mirroring FR-033's confirmation requirement.
// The CLI's own "/chore create" always confirms immediately (matching the
// web UI's create form — see createChore's doc comment), so this seeds an
// unconfirmed Chore directly via the underlying service (as the deferred
// FR-031 chat/agent-proposal path would) to exercise confirm-schedule.
func TestHandleChoreCommand_ConfirmScheduleTwoStepFlow(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	sched, err := domain.NewScheduleFromPreset(domain.SchedulePresetDaily)
	if err != nil {
		t.Fatalf("unexpected error building schedule: %v", err)
	}
	c, err := h.service.CreateChore(ctx, "alice", "Agent proposal", "d", "", sched, false)
	if err != nil {
		t.Fatalf("seed CreateChore failed: %v", err)
	}

	preShow, err := h.HandleChoreCommand(ctx, user, "alice", "/chore show "+c.ID)
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if !strings.Contains(preShow, "Confirmed: false") || !strings.Contains(preShow, "pending confirmation") {
		t.Fatalf("precondition: expected unconfirmed chore, got: %s", preShow)
	}

	confirmResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore confirm-schedule "+c.ID)
	if err != nil {
		t.Fatalf("confirm-schedule failed: %v", err)
	}
	if !strings.Contains(confirmResult, "confirmed") {
		t.Errorf("expected confirmation message, got: %s", confirmResult)
	}

	postShow, err := h.HandleChoreCommand(ctx, user, "alice", "/chore show "+c.ID)
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if !strings.Contains(postShow, "Confirmed: true") {
		t.Errorf("expected chore to be confirmed after confirm-schedule, got: %s", postShow)
	}
	if strings.Contains(postShow, "pending confirmation") {
		t.Errorf("expected a computed next-run time after confirmation, got: %s", postShow)
	}
}

func TestHandleChoreCommand_ConfirmScheduleNotFound(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore confirm-schedule does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected not-found message, got: %s", result)
	}
}

func TestHandleChoreCommand_Delete(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	createResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore create backup d --schedule daily")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractChoreID(t, createResult)

	deleteResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore delete "+id)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(deleteResult, "deleted") {
		t.Errorf("expected deletion confirmation, got: %s", deleteResult)
	}

	showResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore show "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(showResult, "not found") {
		t.Errorf("expected chore to be gone after delete, got: %s", showResult)
	}
}

// TestHandleChoreCommand_DeleteWithActiveRunSoftDeletes proves the CLI
// surfaces spec.md Edge Case #3 / FR-R8's soft-delete semantics (mirroring
// the web UI's Chore/Job delete parity fix) clearly, rather than reporting
// a hard delete that didn't actually happen.
func TestHandleChoreCommand_DeleteWithActiveRunSoftDeletes(t *testing.T) {
	h, runRepo := newTestChoreHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	createResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore create backup d --schedule daily")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractChoreID(t, createResult)

	if err := runRepo.SaveRun(ctx, &domain.Run{
		ID:          "run-1",
		OwnerUserID: "alice",
		SourceType:  domain.SourceTypeChore,
		SourceID:    id,
		Status:      domain.RunStatusRunning,
		CreatedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	deleteResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore delete "+id)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(deleteResult, "active run") {
		t.Errorf("expected active-run message, got: %s", deleteResult)
	}

	showResult, err := h.HandleChoreCommand(ctx, user, "alice", "/chore show "+id)
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if strings.Contains(showResult, "not found") {
		t.Errorf("expected soft-deleted chore to still exist, got: %s", showResult)
	}
	if !strings.Contains(showResult, "pending deletion") {
		t.Errorf("expected pending-deletion status in show output, got: %s", showResult)
	}
}

func TestHandleChoreCommand_DeleteNotFound(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore delete does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected not-found message, got: %s", result)
	}
}

// TestHandleChoreCommand_UsesOwnerUserIDNotCurrentUserID is the single
// highest-value test given AD-5's silent-failure-mode risk: it proves
// ownerUserID (the explicit parameter), not currentUser.ID, is what scopes
// data in the underlying service.
func TestHandleChoreCommand_UsesOwnerUserIDNotCurrentUserID(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	// currentUser.ID is deliberately different from ownerUserID.
	user := &domain.User{ID: "current-user-id", Role: domain.RoleUser}
	ownerUserID := "session-username"

	if _, err := h.HandleChoreCommand(ctx, user, ownerUserID, "/chore create backup d --schedule daily"); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Listing under ownerUserID finds the chore.
	result, err := h.HandleChoreCommand(ctx, user, ownerUserID, "/chore list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "backup") {
		t.Fatalf("expected chore visible under ownerUserID, got: %s", result)
	}

	// Listing under currentUser.ID (the wrong key) must NOT find it — if it
	// did, the handler would have used currentUser.ID instead of
	// ownerUserID somewhere, violating AD-5.
	result, err = h.HandleChoreCommand(ctx, user, user.ID, "/chore list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "backup") {
		t.Fatalf("chore leaked under currentUser.ID instead of ownerUserID: %s", result)
	}
}

func TestHandleChoreCommand_CrossUserIsolation(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	ctx := context.Background()
	userA := &domain.User{ID: "a", Role: domain.RoleUser}
	userB := &domain.User{ID: "b", Role: domain.RoleUser}

	createResult, err := h.HandleChoreCommand(ctx, userA, "alice", "/chore create secret d --schedule daily")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractChoreID(t, createResult)

	// Bob (ownerUserID="bob") cannot see alice's chore.
	showResult, err := h.HandleChoreCommand(ctx, userB, "bob", "/chore show "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(showResult, "not found") {
		t.Fatalf("expected cross-owner show to report not-found, got: %s", showResult)
	}

	// Bob cannot delete alice's chore either.
	deleteResult, err := h.HandleChoreCommand(ctx, userB, "bob", "/chore delete "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(deleteResult, "not found") {
		t.Fatalf("expected cross-owner delete to report not-found, got: %s", deleteResult)
	}

	// Bob cannot confirm alice's chore's schedule either.
	confirmResult, err := h.HandleChoreCommand(ctx, userB, "bob", "/chore confirm-schedule "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(confirmResult, "not found") {
		t.Fatalf("expected cross-owner confirm-schedule to report not-found, got: %s", confirmResult)
	}

	// The chore is still visible to its actual owner, proving Bob's
	// attempts didn't affect it.
	aliceShowResult, err := h.HandleChoreCommand(ctx, userA, "alice", "/chore show "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(aliceShowResult, "not found") {
		t.Fatalf("alice's chore should still exist after bob's failed cross-owner attempts: %s", aliceShowResult)
	}
}

// TestHandleChoreCommand_ChatNotImplemented guards FR-031's deliberate
// deferral: ChoresService has no chat/converse method to mirror, so
// "/chore chat ..." must never grow new CLI-only chat capability. FR-003
// (auto-review fix pass): the response must be a specific "not yet
// implemented" message naming the command and FR-031, distinguishable from
// a genuine typo — not the generic "Unknown chore command" fallthrough.
func TestHandleChoreCommand_ChatNotImplemented(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore chat some-id hello there")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(result), "hello there") {
		t.Fatalf("chat subcommand must not be implemented as new capability, got: %s", result)
	}
	if result != choreChatNotImplementedMessage {
		t.Errorf("expected the specific choreChatNotImplementedMessage, got: %s", result)
	}
	if strings.Contains(result, "Unknown chore command") {
		t.Errorf("expected a specific not-yet-implemented message, not the generic unknown-command fallthrough, got: %s", result)
	}

	// Bare "/chore chat" (no id/message args) must also return the
	// specific message, not fall through to generic showHelp().
	bareResult, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bareResult != choreChatNotImplementedMessage {
		t.Errorf("expected bare '/chore chat' to also return choreChatNotImplementedMessage, got: %s", bareResult)
	}
}

// TestHandleChoreCommand_UnknownSubcommandStillGeneric verifies a genuine
// typo (not "chat") still gets the generic unrecognized-subcommand
// response, so FR-003's fix for "chat" doesn't accidentally swallow real
// typos into a misleading "not yet implemented" message.
func TestHandleChoreCommand_UnknownSubcommandStillGeneric(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleChoreCommand(context.Background(), user, "alice", "/chore chta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Unknown chore command") {
		t.Errorf("expected the generic unknown-subcommand response for a real typo, got: %s", result)
	}
}

func TestHandleChoreCommand_SatisfiesEnvCommandHandler(t *testing.T) {
	h, _ := newTestChoreHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	var envHandler EnvCommandHandler = h
	result, err := envHandler.Handle(context.Background(), user, "alice", "/chore list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No chores found") {
		t.Errorf("expected empty-list message via Handle, got: %s", result)
	}
}
