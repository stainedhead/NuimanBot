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

func newTestService(t *testing.T) *Service {
	t.Helper()
	repo := storage.NewFileProjectRepository(t.TempDir())
	return NewService(repo)
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
	s := newTestService(t)
	ctx := context.Background()
	outDir := filepath.Join(t.TempDir(), "my-project")

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
	s := newTestService(t)
	ctx := context.Background()
	outDir := filepath.Join(t.TempDir(), "my-project")

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
	s := newTestService(t)
	_, err := s.CreateProject(context.Background(), "", "name", filepath.Join(t.TempDir(), "p"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateProject_RequiresName(t *testing.T) {
	s := newTestService(t)
	_, err := s.CreateProject(context.Background(), "user-a", "   ", filepath.Join(t.TempDir(), "p"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateProject_RequiresOutputDirectory(t *testing.T) {
	s := newTestService(t)
	_, err := s.CreateProject(context.Background(), "user-a", "name", "   ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestListProjects_Empty(t *testing.T) {
	s := newTestService(t)
	got, err := s.ListProjects(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v", got)
	}
}

func TestListProjects_OwnerScoped(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	if _, err := s.CreateProject(ctx, "user-a", "A's project", filepath.Join(t.TempDir(), "a")); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := s.CreateProject(ctx, "user-b", "B's project", filepath.Join(t.TempDir(), "b")); err != nil {
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "secret project", filepath.Join(t.TempDir(), "p"))
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "secret project", filepath.Join(t.TempDir(), "p"))
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "delete me", filepath.Join(t.TempDir(), "p"))
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", filepath.Join(t.TempDir(), "p"))
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", filepath.Join(t.TempDir(), "p"))
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

func TestAddAgentsFile_CrossOwnerIsolation(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", filepath.Join(t.TempDir(), "p"))
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
	s := newTestService(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "user-a", "keep me forever", filepath.Join(t.TempDir(), "p")); err != nil {
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "old project", filepath.Join(t.TempDir(), "p"))
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
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "long running project", filepath.Join(t.TempDir(), "p"))
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
	// os.MkdirAll fails because a non-directory occupies that path.
	blockingFile := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	s := newTestService(t)
	_, err := s.CreateProject(context.Background(), "user-a", "proj", blockingFile)
	if err == nil {
		t.Fatal("expected error when output directory path is occupied by a file")
	}
}

func TestCreateProject_HiddenDirectoryMkdirAllFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks are bypassed when running as root")
	}
	outDir := filepath.Join(t.TempDir(), "readonly-project")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to pre-create output dir: %v", err)
	}
	if err := os.Chmod(outDir, 0555); err != nil {
		t.Fatalf("failed to chmod output dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(outDir, 0755) })

	s := newTestService(t)
	_, err := s.CreateProject(context.Background(), "user-a", "proj", outDir)
	if err == nil {
		t.Fatal("expected error creating hidden directory inside a read-only output directory")
	}
}

func TestCreateProject_SaveProjectError(t *testing.T) {
	repo := &errorInjectingRepo{
		inner:   storage.NewFileProjectRepository(t.TempDir()),
		saveErr: errors.New("save boom"),
	}
	s := NewService(repo)
	_, err := s.CreateProject(context.Background(), "user-a", "proj", filepath.Join(t.TempDir(), "p"))
	if err == nil {
		t.Fatal("expected error when SaveProject fails")
	}
}

func TestAddAgentsFile_WriteFileFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks are bypassed when running as root")
	}
	s := newTestService(t)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", filepath.Join(t.TempDir(), "p"))
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
	repo := &errorInjectingRepo{inner: storage.NewFileProjectRepository(t.TempDir())}
	s := NewService(repo)
	ctx := context.Background()

	p, err := s.CreateProject(ctx, "user-a", "proj", filepath.Join(t.TempDir(), "p"))
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
	s := NewService(repo)
	_, err := s.SweepExpired(context.Background(), "user-a", domain.NewRetentionPolicy(24*time.Hour), time.Now())
	if err == nil {
		t.Fatal("expected error when ListProjects fails")
	}
}

func TestSweepExpired_DeleteErrorContinuesSweep(t *testing.T) {
	inner := storage.NewFileProjectRepository(t.TempDir())
	repo := &errorInjectingRepo{inner: inner, deleteErrFor: map[string]error{}}
	s := NewService(repo)
	ctx := context.Background()

	failing, err := s.CreateProject(ctx, "user-a", "failing project", filepath.Join(t.TempDir(), "p1"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	succeeding, err := s.CreateProject(ctx, "user-a", "succeeding project", filepath.Join(t.TempDir(), "p2"))
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
