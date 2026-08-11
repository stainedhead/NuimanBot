package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// newTestService returns a Service plus the allowedRootBase it was
// constructed with, so tests can build a valid outputDirectory for
// CreateProject via projectDir — FR-R18 confines outputDirectory to
// <root>/users/<ownerUserID>/projects/, so an arbitrary unrelated
// t.TempDir() (the pre-fix convention this test file used) is no longer
// accepted.
func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	repo := storage.NewFileProjectRepository(t.TempDir())
	root := t.TempDir()
	return NewService(repo, storage.NewFileConfinedFileStore(), root), root
}

// projectDir builds a valid outputDirectory for ownerUserID under root,
// matching the allowed-root convention CreateProject enforces (FR-R18):
// <root>/users/<ownerUserID>/projects/<name>.
func projectDir(root, ownerUserID, name string) string {
	return filepath.Join(root, "users", ownerUserID, "projects", name)
}

// errorInjectingRepo wraps a real domain.ProjectRepository and lets tests
// force specific method calls to fail, to exercise error paths the
// file-based repository doesn't naturally hit.
type errorInjectingRepo struct {
	inner        domain.ProjectRepository
	saveErr      error
	listErr      error
	deleteErrFor map[string]error
}

func (r *errorInjectingRepo) SaveProject(ctx context.Context, p *domain.Project) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.inner.SaveProject(ctx, p)
}

func (r *errorInjectingRepo) GetProject(ctx context.Context, ownerUserID, projectID string) (*domain.Project, error) {
	return r.inner.GetProject(ctx, ownerUserID, projectID)
}

func (r *errorInjectingRepo) ListProjects(ctx context.Context, ownerUserID string) ([]*domain.Project, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.inner.ListProjects(ctx, ownerUserID)
}

func (r *errorInjectingRepo) DeleteProject(ctx context.Context, ownerUserID, projectID string) error {
	if err, ok := r.deleteErrFor[projectID]; ok {
		return err
	}
	return r.inner.DeleteProject(ctx, ownerUserID, projectID)
}

var _ domain.ProjectRepository = (*errorInjectingRepo)(nil)

func TestCreateProject_Success(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()
	outDir := projectDir(root, "user-a", "my-project")

	p, err := s.CreateProject(ctx, "user-a", "My Project", outDir)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.OwnerUserID != "user-a" {
		t.Fatalf("expected owner user-a, got %q", p.OwnerUserID)
	}
	if p.Name != "My Project" {
		t.Fatalf("expected name %q, got %q", "My Project", p.Name)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if !p.Retention.IsNever() {
		t.Fatal("expected default retention of Never")
	}
}

func TestCreateProject_CreatesDirectoriesOnDisk(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()
	outDir := projectDir(root, "user-a", "my-project")

	p, err := s.CreateProject(ctx, "user-a", "My Project", outDir)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if info, err := os.Stat(p.OutputDirectory); err != nil || !info.IsDir() {
		t.Fatalf("expected OutputDirectory to exist on disk: %v", err)
	}
	if info, err := os.Stat(p.HiddenDirectory); err != nil || !info.IsDir() {
		t.Fatalf("expected HiddenDirectory to exist on disk: %v", err)
	}
	if p.HiddenDirectory == p.OutputDirectory {
		t.Fatal("expected HiddenDirectory to differ from OutputDirectory")
	}
}

func TestCreateProject_RequiresOwnerUserID(t *testing.T) {
	s, root := newTestService(t)
	_, err := s.CreateProject(context.Background(), "", "name", projectDir(root, "user-a", "p"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateProject_RequiresName(t *testing.T) {
	s, root := newTestService(t)
	_, err := s.CreateProject(context.Background(), "user-a", "   ", projectDir(root, "user-a", "p"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateProject_RequiresOutputDirectory(t *testing.T) {
	s, _ := newTestService(t)
	_, err := s.CreateProject(context.Background(), "user-a", "name", "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// --- FR-R18: outputDirectory confinement to the caller's own allowed root ---

func TestCreateProject_RejectsRelativeTraversalOutsideAllowedRoot(t *testing.T) {
	s, root := newTestService(t)
	outDir := filepath.Join(root, "users", "user-a", "projects", "..", "..", "..", "data")

	_, err := s.CreateProject(context.Background(), "user-a", "escape attempt", outDir)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for relative-traversal escape, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(statErr) {
		t.Fatal("expected no directory to have been created outside the allowed root")
	}
}

func TestCreateProject_RejectsAbsolutePathOutsideAllowedRoot(t *testing.T) {
	s, root := newTestService(t)
	outside := filepath.Join(filepath.Dir(root), "outside-someone-elses-data")

	_, err := s.CreateProject(context.Background(), "user-a", "escape attempt", outside)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for absolute path outside the allowed root, got %v", err)
	}
	if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
		t.Fatal("expected no directory to have been created outside the allowed root")
	}
}

func TestCreateProject_RejectsAnotherUsersAllowedRoot(t *testing.T) {
	// Requesting user-b is confined to <root>/users/user-b/projects/, so an
	// outputDirectory under user-a's tree — even though it's still lexically
	// inside the shared storage root overall — must be rejected too.
	s, root := newTestService(t)
	outDir := projectDir(root, "user-a", "p")

	_, err := s.CreateProject(context.Background(), "user-b", "cross-user attempt", outDir)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for another user's allowed root, got %v", err)
	}
}

func TestCreateProject_AllowsPathWithinOwnAllowedRoot(t *testing.T) {
	s, root := newTestService(t)
	outDir := projectDir(root, "user-a", "legit-project")

	p, err := s.CreateProject(context.Background(), "user-a", "legit", outDir)
	if err != nil {
		t.Fatalf("expected in-root outputDirectory to be allowed, got err: %v", err)
	}
	if p.OutputDirectory != outDir {
		t.Fatalf("expected OutputDirectory %q, got %q", outDir, p.OutputDirectory)
	}
}

func TestListProjects_Empty(t *testing.T) {
	s, _ := newTestService(t)
	got, err := s.ListProjects(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v", got)
	}
}

func TestListProjects_OwnerScoped(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	if _, err := s.CreateProject(ctx, "user-a", "A's project", projectDir(root, "user-a", "a")); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := s.CreateProject(ctx, "user-b", "B's project", projectDir(root, "user-b", "b")); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, err := s.ListProjects(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 1 || got[0].OwnerUserID != "user-a" {
		t.Fatalf("expected only user-a's project, got %+v", got)
	}
}

func TestGetProject_CrossOwnerIsolation(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "secret project", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if _, err := s.GetProject(ctx, "user-b", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner access, got %v", err)
	}
	if got, err := s.GetProject(ctx, "user-a", p.ID); err != nil || got.ID != p.ID {
		t.Fatalf("expected owner to retrieve their own project, got %v, err %v", got, err)
	}
}

func TestDeleteProject_CrossOwnerIsolation(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "secret project", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.DeleteProject(ctx, "user-b", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner delete, got %v", err)
	}
	if _, err := s.GetProject(ctx, "user-a", p.ID); err != nil {
		t.Fatalf("expected project to still exist: %v", err)
	}
}

func TestDeleteProject_Success(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "delete me", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := s.DeleteProject(ctx, "user-a", p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := s.GetProject(ctx, "user-a", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestAddAgentsFile_CreatesFile(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err != nil {
		t.Fatalf("AddAgentsFile: %v", err)
	}

	agentsPath := filepath.Join(p.OutputDirectory, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err != nil {
		t.Fatalf("expected AGENTS.md to exist: %v", err)
	}

	updated, err := s.GetProject(ctx, "user-a", p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if !updated.UpdatedAt.After(p.CreatedAt) && !updated.UpdatedAt.Equal(p.CreatedAt) {
		t.Fatalf("expected UpdatedAt to be refreshed, got %v (created %v)", updated.UpdatedAt, p.CreatedAt)
	}
}

func TestAddAgentsFile_Idempotent(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err != nil {
		t.Fatalf("AddAgentsFile (first): %v", err)
	}

	agentsPath := filepath.Join(p.OutputDirectory, "AGENTS.md")
	customContent := []byte("# user-edited content\n")
	if err := os.WriteFile(agentsPath, customContent, 0644); err != nil {
		t.Fatalf("failed to simulate user edit: %v", err)
	}

	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err != nil {
		t.Fatalf("AddAgentsFile (second): %v", err)
	}

	got, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("failed to read AGENTS.md: %v", err)
	}
	if string(got) != string(customContent) {
		t.Fatalf("expected idempotent AddAgentsFile to preserve existing content, got %q", got)
	}
}

func TestAddAgentsFile_RejectsOutputDirectoryOutsideAllowedRoot(t *testing.T) {
	// Defense in depth for FR-R18 (flagged by second-reviewer pass):
	// CreateProject validates outputDirectory against the allowed root at
	// creation time, but a Project record's OutputDirectory could still
	// end up outside it later — a pre-fix record, or the on-disk
	// directory replaced with a symlink escaping the root (FR-022 grants
	// the user direct filesystem access to it). AddAgentsFile must not
	// blindly trust the persisted value on every subsequent use.
	repo := storage.NewFileProjectRepository(t.TempDir())
	root := t.TempDir()
	s := NewService(repo, storage.NewFileConfinedFileStore(), root)
	ctx := context.Background()

	outside := t.TempDir() // deliberately not under root/users/user-a/projects
	p := &domain.Project{
		ID:              "escaped-project",
		OwnerUserID:     "user-a",
		Name:            "escaped",
		OutputDirectory: outside,
		HiddenDirectory: filepath.Join(outside, ".nuimanbot"),
		Retention:       domain.NeverExpire(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := repo.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err == nil {
		t.Fatal("expected AddAgentsFile to reject a Project whose OutputDirectory is outside the allowed root")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "AGENTS.md")); !os.IsNotExist(statErr) {
		t.Fatal("expected no AGENTS.md to have been written outside the allowed root")
	}
}

func TestAddAgentsFile_CrossOwnerIsolation(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.AddAgentsFile(ctx, "user-b", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner AddAgentsFile, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.OutputDirectory, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("expected AGENTS.md to not be created by a cross-owner call")
	}
}

func TestSweepExpired_NeverPolicyDeletesNothing(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "user-a", "keep me forever", projectDir(root, "user-a", "p")); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	n, err := s.SweepExpired(ctx, "user-a", domain.NeverExpire(), time.Now().Add(100*365*24*time.Hour))
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 deletions under Never policy, got %d", n)
	}
}

func TestSweepExpired_DeletesExpiredOnly(t *testing.T) {
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "old project", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	policy := domain.NewRetentionPolicy(24 * time.Hour)
	now := time.Now()

	n, err := s.SweepExpired(ctx, "user-a", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 deletions immediately after creation, got %d", n)
	}

	n, err = s.SweepExpired(ctx, "user-a", policy, now.Add(25*time.Hour))
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deletion 25h later, got %d", n)
	}

	if _, err := s.GetProject(ctx, "user-a", p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected project deleted, got %v", err)
	}
}

func TestSweepExpired_ActiveProjectSurvivesViaUpdatedAt(t *testing.T) {
	// Edge Case #12's Chat analogue, applied to Projects: an old Project
	// with recent activity (AddAgentsFile refreshes UpdatedAt) must not be
	// auto-deleted.
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "long running project", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	policy := domain.NewRetentionPolicy(24 * time.Hour)

	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err != nil {
		t.Fatalf("AddAgentsFile: %v", err)
	}

	n, err := s.SweepExpired(ctx, "user-a", policy, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected active project to survive the sweep, got %d deletions", n)
	}
}

func TestCreateProject_OutputDirectoryMkdirAllFailure(t *testing.T) {
	// Pre-create a regular file where the output directory should go, so
	// os.MkdirAll fails because a non-directory occupies that path. The
	// blocking file itself must be within the allowed root, so this
	// exercises the MkdirAll failure path rather than being short-circuited
	// by the (separate) FR-R18 confinement check.
	s, root := newTestService(t)
	blockingFile := projectDir(root, "user-a", "blocked")
	if err := os.MkdirAll(filepath.Dir(blockingFile), 0755); err != nil {
		t.Fatalf("failed to set up test fixture: %v", err)
	}
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	_, err := s.CreateProject(context.Background(), "user-a", "proj", blockingFile)
	if err == nil {
		t.Fatal("expected error when output directory path is occupied by a file")
	}
}

func TestCreateProject_HiddenDirectoryMkdirAllFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks are bypassed when running as root")
	}
	s, root := newTestService(t)
	outDir := projectDir(root, "user-a", "readonly-project")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to pre-create output dir: %v", err)
	}
	if err := os.Chmod(outDir, 0555); err != nil {
		t.Fatalf("failed to chmod output dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(outDir, 0755) })

	_, err := s.CreateProject(context.Background(), "user-a", "proj", outDir)
	if err == nil {
		t.Fatal("expected error creating hidden directory inside a read-only output directory")
	}
}

func TestCreateProject_SaveProjectError(t *testing.T) {
	root := t.TempDir()
	repo := &errorInjectingRepo{
		inner:   storage.NewFileProjectRepository(t.TempDir()),
		saveErr: errors.New("save boom"),
	}
	s := NewService(repo, storage.NewFileConfinedFileStore(), root)
	_, err := s.CreateProject(context.Background(), "user-a", "proj", projectDir(root, "user-a", "p"))
	if err == nil {
		t.Fatal("expected error when SaveProject fails")
	}
}

func TestAddAgentsFile_WriteFileFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks are bypassed when running as root")
	}
	s, root := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := os.Chmod(p.OutputDirectory, 0555); err != nil {
		t.Fatalf("failed to chmod output dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(p.OutputDirectory, 0755) })

	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err == nil {
		t.Fatal("expected error writing AGENTS.md into a read-only output directory")
	}
}

func TestAddAgentsFile_SaveProjectError(t *testing.T) {
	root := t.TempDir()
	repo := &errorInjectingRepo{inner: storage.NewFileProjectRepository(t.TempDir())}
	s := NewService(repo, storage.NewFileConfinedFileStore(), root)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", projectDir(root, "user-a", "p"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	repo.saveErr = errors.New("save boom")
	if err := s.AddAgentsFile(ctx, "user-a", p.ID); err == nil {
		t.Fatal("expected error when SaveProject fails after writing AGENTS.md")
	}

	if _, err := os.Stat(filepath.Join(p.OutputDirectory, "AGENTS.md")); err != nil {
		t.Fatalf("expected AGENTS.md to have been written despite the save failure: %v", err)
	}
}

func TestSweepExpired_ListProjectsError(t *testing.T) {
	repo := &errorInjectingRepo{
		inner:   storage.NewFileProjectRepository(t.TempDir()),
		listErr: errors.New("list boom"),
	}
	s := NewService(repo, storage.NewFileConfinedFileStore(), t.TempDir())
	_, err := s.SweepExpired(context.Background(), "user-a", domain.NewRetentionPolicy(24*time.Hour), time.Now())
	if err == nil {
		t.Fatal("expected error when ListProjects fails")
	}
}

func TestSweepExpired_DeleteErrorContinuesSweep(t *testing.T) {
	root := t.TempDir()
	inner := storage.NewFileProjectRepository(t.TempDir())
	repo := &errorInjectingRepo{inner: inner, deleteErrFor: map[string]error{}}
	s := NewService(repo, storage.NewFileConfinedFileStore(), root)
	ctx := context.Background()

	failing, err := s.CreateProject(ctx, "user-a", "failing project", projectDir(root, "user-a", "p1"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	succeeding, err := s.CreateProject(ctx, "user-a", "succeeding project", projectDir(root, "user-a", "p2"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	repo.deleteErrFor[failing.ID] = errors.New("delete boom")

	policy := domain.NewRetentionPolicy(24 * time.Hour)
	n, err := s.SweepExpired(ctx, "user-a", policy, time.Now().Add(25*time.Hour))
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 successful deletion despite the other failing, got %d", n)
	}
	if _, err := inner.GetProject(ctx, "user-a", succeeding.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected succeeding project deleted, got %v", err)
	}
	if _, err := inner.GetProject(ctx, "user-a", failing.ID); err != nil {
		t.Fatalf("expected failing project to still exist after its delete errored: %v", err)
	}
}
