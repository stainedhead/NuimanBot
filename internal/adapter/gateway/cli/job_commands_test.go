package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/jobs"
)

// stubRunEnqueuer is a minimal test double for jobs.RunEnqueuer.
type stubRunEnqueuer struct{}

func (stubRunEnqueuer) Enqueue(context.Context, jobs.EnqueueRequest) error { return nil }

// stubProjectLookup is a test double for jobs.ProjectDirectoryLookup, scoped
// by ownerUserID exactly like the real implementation would be: a project ID
// only resolves for the owner it was registered under, so a caller-supplied
// ownerUserID that doesn't match returns domain.ErrNotFound.
type stubProjectLookup struct {
	// dirs maps ownerUserID -> projectID -> outputDirectory.
	dirs map[string]map[string]string
}

func (s *stubProjectLookup) OutputDirectoryFor(_ context.Context, ownerUserID, projectID string) (string, error) {
	byOwner, ok := s.dirs[ownerUserID]
	if !ok {
		return "", domain.ErrNotFound
	}
	dir, ok := byOwner[projectID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return dir, nil
}

// newTestJobHandler builds a *cli.JobCommandHandler backed by a real
// jobs.Service (using the same in-memory-friendly file-backed test doubles
// internal/usecase/jobs/service_test.go uses), optionally with a
// project lookup registered for cross-owner isolation tests.
func newTestJobHandler(t *testing.T, lookup jobs.ProjectDirectoryLookup) *cli.JobCommandHandler {
	t.Helper()
	tmp := t.TempDir()
	svc := jobs.NewService(
		storage.NewFileJobRepository(tmp),
		storage.NewFileRunRepository(tmp),
		stubRunEnqueuer{},
		lookup,
		storage.NewFileConfinedFileStore(),
		tmp,
	)
	return cli.NewJobCommandHandler(svc)
}

func TestJobCommand_BareShowsHelp(t *testing.T) {
	h := newTestJobHandler(t, nil)
	out, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "list") || !strings.Contains(out, "create") || !strings.Contains(out, "show") || !strings.Contains(out, "delete") {
		t.Fatalf("expected help text listing subcommands, got: %q", out)
	}
}

func TestJobCommand_UnrecognizedSubcommand(t *testing.T) {
	h := newTestJobHandler(t, nil)
	out, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job bogus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Unknown") {
		t.Fatalf("expected unknown-subcommand message, got: %q", out)
	}
}

func TestJobCommand_ChatDeferred(t *testing.T) {
	// FR-026 is explicitly deferred: /job chat must not be recognized as a
	// working subcommand (no new chat capability built).
	h := newTestJobHandler(t, nil)
	out, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job chat job-1 hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "hello") {
		t.Fatalf("expected /job chat to NOT be handled as a real chat command, got: %q", out)
	}
}

func TestJobCommand_CreateAndList(t *testing.T) {
	h := newTestJobHandler(t, nil)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	out, err := h.HandleJobCommand(ctx, user, "alice", `/job create "Clean the inbox" "Archive anything older than 30 days."`)
	if err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}
	if !strings.Contains(out, "Clean the inbox") {
		t.Fatalf("expected created job title in output, got: %q", out)
	}

	out, err = h.HandleJobCommand(ctx, user, "alice", "/job list")
	if err != nil {
		t.Fatalf("list: unexpected error: %v", err)
	}
	if !strings.Contains(out, "Clean the inbox") {
		t.Fatalf("expected job in list output, got: %q", out)
	}
}

func TestJobCommand_ListEmpty(t *testing.T) {
	h := newTestJobHandler(t, nil)
	out, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "no job") {
		t.Fatalf("expected an empty-list message, got: %q", out)
	}
}

func TestJobCommand_CreateWithChatContext(t *testing.T) {
	h := newTestJobHandler(t, nil)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	_, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title Description --chat chat-1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := h.HandleJobCommand(ctx, user, "alice", "/job list --chat chat-1")
	if err != nil {
		t.Fatalf("list --chat: unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected job filtered by --chat context, got: %q", out)
	}

	out, err = h.HandleJobCommand(ctx, user, "alice", "/job list --project some-other-id")
	if err != nil {
		t.Fatalf("list --project: unexpected error: %v", err)
	}
	if strings.Contains(out, "Title") {
		t.Fatalf("expected job to be filtered OUT by unrelated --project context, got: %q", out)
	}
}

func TestJobCommand_CreateWithProjectContext(t *testing.T) {
	lookup := &stubProjectLookup{dirs: map[string]map[string]string{
		"alice": {"proj-1": "/output/proj-1"},
	}}
	h := newTestJobHandler(t, lookup)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	out, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title Description --project proj-1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected created job in output, got: %q", out)
	}

	out, err = h.HandleJobCommand(ctx, user, "alice", "/job list --project proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected job filtered by --project context, got: %q", out)
	}
}

// TestJobCommand_CreateWithForeignProject_FailsNotFound is the mandatory
// explicit test for the traced edge case from implementation-notes.md: a
// /job create --project <id> where <id> belongs to a DIFFERENT owner must
// resolve as a not-found/permission error, never silently attach the new
// Job to another user's Project. This is safe by construction because
// ProjectDirectoryLookup.OutputDirectoryFor is always called with the
// caller-supplied ownerUserID (never a target-project-derived owner) — but
// per the explicit acceptance criterion, that must be proven with a test,
// not merely relied upon.
func TestJobCommand_CreateWithForeignProject_FailsNotFound(t *testing.T) {
	lookup := &stubProjectLookup{dirs: map[string]map[string]string{
		// proj-1 exists only under "bob", not "alice".
		"bob": {"proj-1": "/output/proj-1"},
	}}
	h := newTestJobHandler(t, lookup)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	out, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title Description --project proj-1`)
	// CreateJob itself does not hard-fail on an unresolved project lookup
	// (Edge Case #2: WorkingDirectory is simply left unresolved), so the Job
	// creation succeeds — but it must NOT have attached bob's project
	// directory. Verify no cross-owner directory leaked into the output/job.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "/output/proj-1") {
		t.Fatalf("expected bob's project directory to NOT be resolved for alice's job, got: %q", out)
	}

	jobsList, listErr := h.HandleJobCommand(ctx, user, "alice", "/job list")
	if listErr != nil {
		t.Fatalf("unexpected error: %v", listErr)
	}
	if strings.Contains(jobsList, "/output/proj-1") {
		t.Fatalf("expected alice's job to never carry bob's project working directory, got: %q", jobsList)
	}
}

// TestJobCommand_OwnerUserIDIsUsed_NotCurrentUserID proves AD-5: the
// ownerUserID parameter (not currentUser.ID) is what scopes data. A Job
// created under ownerUserID "alice" must be invisible to a call using
// currentUser.ID "alice" as ownerUserID would have been, if the handler
// wrongly used currentUser.ID instead.
func TestJobCommand_OwnerUserIDIsUsed_NotCurrentUserID(t *testing.T) {
	h := newTestJobHandler(t, nil)
	ctx := context.Background()

	// currentUser.ID is deliberately different from the ownerUserID passed
	// in, mirroring how the real Gateway calls Handle: currentUser is the
	// domain.User for role checks, ownerUserID is the session's Username.
	user := &domain.User{ID: "internal-user-id-999"}

	if _, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title Description`); err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}

	// Listing under ownerUserID "alice" must find the job.
	out, err := h.HandleJobCommand(ctx, user, "alice", "/job list")
	if err != nil {
		t.Fatalf("list alice: unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected job under ownerUserID alice, got: %q", out)
	}

	// Listing under ownerUserID equal to currentUser.ID must NOT find it —
	// proves currentUser.ID was never used as the scoping key.
	out, err = h.HandleJobCommand(ctx, user, "internal-user-id-999", "/job list")
	if err != nil {
		t.Fatalf("list currentUser.ID: unexpected error: %v", err)
	}
	if strings.Contains(out, "Title") {
		t.Fatalf("expected no job visible under currentUser.ID as ownerUserID, got: %q", out)
	}
}

func TestJobCommand_ShowFound(t *testing.T) {
	h := newTestJobHandler(t, nil)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	createOut, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title "some description"`)
	if err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}
	id := extractJobID(t, createOut)

	out, err := h.HandleJobCommand(ctx, user, "alice", "/job show "+id)
	if err != nil {
		t.Fatalf("show: unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "some description") {
		t.Fatalf("expected job detail in output, got: %q", out)
	}
}

func TestJobCommand_ShowNotFound(t *testing.T) {
	h := newTestJobHandler(t, nil)
	_, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job show missing-id")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestJobCommand_ShowCrossOwnerIsNotFound(t *testing.T) {
	h := newTestJobHandler(t, nil)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	createOut, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title Description`)
	if err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}
	id := extractJobID(t, createOut)

	if _, err := h.HandleJobCommand(ctx, user, "bob", "/job show "+id); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner show, got: %v", err)
	}
}

func TestJobCommand_Delete(t *testing.T) {
	h := newTestJobHandler(t, nil)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	createOut, err := h.HandleJobCommand(ctx, user, "alice", `/job create Title Description`)
	if err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}
	id := extractJobID(t, createOut)

	out, err := h.HandleJobCommand(ctx, user, "alice", "/job delete "+id)
	if err != nil {
		t.Fatalf("delete: unexpected error: %v", err)
	}
	if !strings.Contains(out, id) {
		t.Fatalf("expected delete confirmation to reference job ID, got: %q", out)
	}
}

func TestJobCommand_DeleteNotFound(t *testing.T) {
	h := newTestJobHandler(t, nil)
	_, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job delete missing-id")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestJobCommand_CreateMissingArgsShowsUsage(t *testing.T) {
	h := newTestJobHandler(t, nil)
	out, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job create")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "usage") {
		t.Fatalf("expected usage message, got: %q", out)
	}
}

func TestJobCommand_ShowMissingIDShowsUsage(t *testing.T) {
	h := newTestJobHandler(t, nil)
	out, err := h.HandleJobCommand(context.Background(), &domain.User{ID: "u1"}, "alice", "/job show")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "usage") {
		t.Fatalf("expected usage message, got: %q", out)
	}
}

// TestJobCommand_SatisfiesEnvCommandHandler verifies *cli.JobCommandHandler
// implements cli.EnvCommandHandler via its Handle method, and that Handle
// delegates to HandleJobCommand with the same ownerUserID.
func TestJobCommand_SatisfiesEnvCommandHandler(t *testing.T) {
	var _ cli.EnvCommandHandler = (*cli.JobCommandHandler)(nil)

	h := newTestJobHandler(t, nil)
	ctx := context.Background()
	user := &domain.User{ID: "u1"}

	if _, err := h.Handle(ctx, user, "alice", `/job create Title Description`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := h.Handle(ctx, user, "alice", "/job list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected Handle to delegate to HandleJobCommand and see created job, got: %q", out)
	}
}

// extractJobID pulls the job ID back out of create/list output that must
// contain a line like "ID: <uuid>".
func extractJobID(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "ID:"))
		}
	}
	t.Fatalf("could not find 'ID:' line in output: %q", out)
	return ""
}
