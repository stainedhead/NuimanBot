package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/projects"
)

// newTestProjectHandler returns a ProjectCommandHandler backed by a real
// projects.Service (file-based repo/store, per internal/usecase/projects's
// own test convention), plus the allowedRootBase it was constructed with so
// tests can build a valid outputDirectory via testProjectDir.
func newTestProjectHandler(t *testing.T) (*ProjectCommandHandler, string) {
	t.Helper()
	repo := storage.NewFileProjectRepository(t.TempDir())
	root := t.TempDir()
	service := projects.NewService(repo, storage.NewFileConfinedFileStore(), root)
	return NewProjectCommandHandler(service), root
}

// testProjectDir builds a valid outputDirectory for ownerUserID under root,
// matching CreateProject's allowed-root confinement (FR-R18):
// <root>/users/<ownerUserID>/projects/<name>.
func testProjectDir(root, ownerUserID, name string) string {
	return filepath.Join(root, "users", ownerUserID, "projects", name)
}

// extractProjectID pulls the "ID: <uuid>" value out of createProject's
// formatted output.
func extractProjectID(t *testing.T, createOutput string) string {
	t.Helper()
	for _, line := range strings.Split(createOutput, "\n") {
		if strings.HasPrefix(line, "ID: ") {
			return strings.TrimPrefix(line, "ID: ")
		}
	}
	t.Fatalf("no ID line found in create output: %q", createOutput)
	return ""
}

func TestHandleProjectCommand_BareShowsHelp(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Project Commands") {
		t.Errorf("expected help text, got: %s", result)
	}
}

func TestHandleProjectCommand_ListEmpty(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No projects found") {
		t.Errorf("expected empty-list message, got: %s", result)
	}
}

func TestHandleProjectCommand_CreateAndList(t *testing.T) {
	h, root := newTestProjectHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}
	outDir := testProjectDir(root, "alice", "proj1")

	createResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project create proj1 "+outDir)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !strings.Contains(createResult, "proj1") || !strings.Contains(createResult, outDir) {
		t.Errorf("unexpected create output: %s", createResult)
	}

	listResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(listResult, "proj1") {
		t.Errorf("expected proj1 in list output, got: %s", listResult)
	}
}

func TestHandleProjectCommand_CreateOutputDirectoryConfinementFailure(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	// Outside the allowed root entirely.
	outsideDir := filepath.Join(t.TempDir(), "elsewhere")
	_, err := h.HandleProjectCommand(ctx, user, "alice", "/project create proj1 "+outsideDir)
	if err == nil {
		t.Fatal("expected an error for an outputDirectory outside the allowed root, got nil")
	}
	if !strings.Contains(err.Error(), "within your own projects root") {
		t.Errorf("expected confinement error to be surfaced clearly, got: %v", err)
	}
}

func TestHandleProjectCommand_CreateUsage(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project create proj1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Usage:") {
		t.Errorf("expected usage message, got: %s", result)
	}
}

func TestHandleProjectCommand_ShowFound(t *testing.T) {
	h, root := newTestProjectHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}
	outDir := testProjectDir(root, "alice", "proj1")

	createResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project create proj1 "+outDir)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractProjectID(t, createResult)

	showResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project show "+id)
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if !strings.Contains(showResult, outDir) {
		t.Errorf("expected output directory in show output, got: %s", showResult)
	}
	if !strings.Contains(showResult, "AGENTS.md: not present") {
		t.Errorf("expected AGENTS.md not-present status, got: %s", showResult)
	}
	if !strings.Contains(showResult, "Retention: Never") {
		t.Errorf("expected retention setting, got: %s", showResult)
	}
}

func TestHandleProjectCommand_ShowNotFound(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project show does-not-exist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected not-found message, got: %s", result)
	}
}

func TestHandleProjectCommand_AddAgentsFile(t *testing.T) {
	h, root := newTestProjectHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}
	outDir := testProjectDir(root, "alice", "proj1")

	createResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project create proj1 "+outDir)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractProjectID(t, createResult)

	addResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project add-agents-file "+id)
	if err != nil {
		t.Fatalf("add-agents-file failed: %v", err)
	}
	if !strings.Contains(addResult, "AGENTS.md") {
		t.Errorf("expected confirmation message, got: %s", addResult)
	}

	showResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project show "+id)
	if err != nil {
		t.Fatalf("show failed: %v", err)
	}
	if !strings.Contains(showResult, "AGENTS.md: present") {
		t.Errorf("expected AGENTS.md present after add-agents-file, got: %s", showResult)
	}
}

func TestHandleProjectCommand_Delete(t *testing.T) {
	h, root := newTestProjectHandler(t)
	ctx := context.Background()
	user := &domain.User{ID: "u1", Role: domain.RoleUser}
	outDir := testProjectDir(root, "alice", "proj1")

	createResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project create proj1 "+outDir)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractProjectID(t, createResult)

	deleteResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project delete "+id)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(deleteResult, "deleted") {
		t.Errorf("expected deletion confirmation, got: %s", deleteResult)
	}

	showResult, err := h.HandleProjectCommand(ctx, user, "alice", "/project show "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(showResult, "not found") {
		t.Errorf("expected project to be gone after delete, got: %s", showResult)
	}
}

// TestHandleProjectCommand_UsesOwnerUserIDNotCurrentUserID is the
// single highest-value test given AD-5's silent-failure-mode risk: it
// proves ownerUserID (the explicit parameter), not currentUser.ID, is what
// scopes data in the underlying service.
func TestHandleProjectCommand_UsesOwnerUserIDNotCurrentUserID(t *testing.T) {
	h, root := newTestProjectHandler(t)
	ctx := context.Background()
	// currentUser.ID is deliberately different from ownerUserID.
	user := &domain.User{ID: "current-user-id", Role: domain.RoleUser}
	ownerUserID := "session-username"
	outDir := testProjectDir(root, ownerUserID, "proj1")

	if _, err := h.HandleProjectCommand(ctx, user, ownerUserID, "/project create proj1 "+outDir); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Listing under ownerUserID finds the project.
	result, err := h.HandleProjectCommand(ctx, user, ownerUserID, "/project list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "proj1") {
		t.Fatalf("expected project visible under ownerUserID, got: %s", result)
	}

	// Listing under currentUser.ID (the wrong key) must NOT find it — if it
	// did, the handler would have used currentUser.ID instead of
	// ownerUserID somewhere, violating AD-5.
	result, err = h.HandleProjectCommand(ctx, user, user.ID, "/project list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "proj1") {
		t.Fatalf("project leaked under currentUser.ID instead of ownerUserID: %s", result)
	}
}

func TestHandleProjectCommand_CrossUserIsolation(t *testing.T) {
	h, root := newTestProjectHandler(t)
	ctx := context.Background()
	userA := &domain.User{ID: "a", Role: domain.RoleUser}
	userB := &domain.User{ID: "b", Role: domain.RoleUser}
	outDir := testProjectDir(root, "alice", "secret")

	createResult, err := h.HandleProjectCommand(ctx, userA, "alice", "/project create secret "+outDir)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	id := extractProjectID(t, createResult)

	// Bob (ownerUserID="bob") cannot see alice's project.
	showResult, err := h.HandleProjectCommand(ctx, userB, "bob", "/project show "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(showResult, "not found") {
		t.Fatalf("expected cross-owner show to report not-found, got: %s", showResult)
	}

	// Bob cannot delete alice's project either.
	deleteResult, err := h.HandleProjectCommand(ctx, userB, "bob", "/project delete "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(deleteResult, "not found") {
		t.Fatalf("expected cross-owner delete to report not-found, got: %s", deleteResult)
	}

	// The project is still visible to its actual owner, proving Bob's
	// attempt didn't delete it.
	aliceShowResult, err := h.HandleProjectCommand(ctx, userA, "alice", "/project show "+id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(aliceShowResult, "not found") {
		t.Fatalf("alice's project should still exist after bob's failed cross-owner delete: %s", aliceShowResult)
	}
}

// TestHandleProjectCommand_ChatNotImplemented guards FR-020's deliberate
// deferral: ProjectsService has no chat/converse method to mirror, so
// "/project chat ..." must never grow new CLI-only chat capability. FR-003
// (auto-review fix pass): the response must be a specific "not yet
// implemented" message naming the command and FR-020, distinguishable from
// a genuine typo — not the generic "Unknown project command" fallthrough.
func TestHandleProjectCommand_ChatNotImplemented(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project chat some-id hello there")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.ToLower(result), "hello there") {
		t.Fatalf("chat subcommand must not be implemented as new capability, got: %s", result)
	}
	if result != projectChatNotImplementedMessage {
		t.Errorf("expected the specific projectChatNotImplementedMessage, got: %s", result)
	}
	if strings.Contains(result, "Unknown project command") {
		t.Errorf("expected a specific not-yet-implemented message, not the generic unknown-command fallthrough, got: %s", result)
	}

	// Bare "/project chat" (no id/message args) — a user typing just this
	// to discover what the subcommand does is exactly the scenario FR-003
	// exists for; it must not fall through to generic showHelp() either.
	bareResult, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bareResult != projectChatNotImplementedMessage {
		t.Errorf("expected bare '/project chat' to also return projectChatNotImplementedMessage, got: %s", bareResult)
	}
}

// TestHandleProjectCommand_UnknownSubcommandStillGeneric verifies a genuine
// typo (not "chat") still gets the generic unrecognized-subcommand
// response, so FR-003's fix for "chat" doesn't accidentally swallow real
// typos into a misleading "not yet implemented" message.
func TestHandleProjectCommand_UnknownSubcommandStillGeneric(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := h.HandleProjectCommand(context.Background(), user, "alice", "/project chta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Unknown project command") {
		t.Errorf("expected the generic unknown-subcommand response for a real typo, got: %s", result)
	}
}

func TestHandleProjectCommand_SatisfiesEnvCommandHandler(t *testing.T) {
	h, _ := newTestProjectHandler(t)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	var envHandler EnvCommandHandler = h
	result, err := envHandler.Handle(context.Background(), user, "alice", "/project list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "No projects found") {
		t.Errorf("expected empty-list message via Handle, got: %s", result)
	}
}
