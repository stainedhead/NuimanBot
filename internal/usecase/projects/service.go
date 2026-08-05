// Package projects orchestrates the web admin's Projects environment
// (FR-017–FR-023): create/list/get/delete a durable, directory-scoped
// Project backed by domain.ProjectRepository, plus AGENTS.md management
// (FR-019–FR-021) and retention (FR-023). Modeled closely on
// internal/usecase/chats for consistency, per spec.md's guidance to model
// net-new entities on the existing Conversation pattern.
package projects

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

// hiddenDirName is the dot-prefixed subdirectory created inside a Project's
// OutputDirectory to hold agent-managed memory/context files (FR-018).
//
// Convention: HiddenDirectory is a literal subdirectory of OutputDirectory
// rather than a path under some separate backend storage root. This keeps
// CreateProject self-contained (it needs no additional storage-root wiring
// beyond the OutputDirectory the caller supplies) while still satisfying
// "never shown in the Project's file view": dot-prefixed directories are
// conventionally hidden from typical file browsers and from `ls` without
// `-a`, and any future Project file-view feature can trivially filter
// entries matching this exact name.
const hiddenDirName = ".nuimanbot"

// Service orchestrates Project create/list/get/delete/AGENTS.md management.
type Service struct {
	projects domain.ProjectRepository
	now      func() time.Time
}

// NewService creates a Projects Service backed by projects.
func NewService(projects domain.ProjectRepository) *Service {
	return &Service{projects: projects, now: time.Now}
}

// CreateProject creates a new Project (FR-017), creating its OutputDirectory
// and HiddenDirectory (FR-018) on disk. Retention defaults to "Never";
// per-user configuration of Project retention (FR-023) is a Settings
// environment concern, out of scope for this Service.
func (s *Service) CreateProject(ctx context.Context, ownerUserID, name, outputDirectory string) (*domain.Project, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(outputDirectory) == "" {
		return nil, fmt.Errorf("%w: outputDirectory is required", domain.ErrInvalidInput)
	}

	absOutput, err := filepath.Abs(filepath.Clean(outputDirectory))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid outputDirectory: %v", domain.ErrInvalidInput, err)
	}
	hiddenDir := filepath.Join(absOutput, hiddenDirName)

	if err := os.MkdirAll(absOutput, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create hidden directory: %w", err)
	}

	now := s.now()
	p := &domain.Project{
		ID:              uuid.NewString(),
		OwnerUserID:     ownerUserID,
		Name:            name,
		OutputDirectory: absOutput,
		HiddenDirectory: hiddenDir,
		Retention:       domain.NeverExpire(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.projects.SaveProject(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to save project: %w", err)
	}
	return p, nil
}

// ListProjects returns ownerUserID's Projects.
func (s *Service) ListProjects(ctx context.Context, ownerUserID string) ([]*domain.Project, error) {
	return s.projects.ListProjects(ctx, ownerUserID)
}

// GetProject retrieves a Project by ID, scoped to its owner. Cross-owner
// access resolves as domain.ErrNotFound (enforced by the repository layer;
// see domain.ProjectRepository's doc comment).
func (s *Service) GetProject(ctx context.Context, ownerUserID, projectID string) (*domain.Project, error) {
	return s.projects.GetProject(ctx, ownerUserID, projectID)
}

// DeleteProject deletes a Project by ID, scoped to its owner (FR-023's
// manual-delete counterpart). Per spec.md Edge Case #2, this Service has no
// knowledge of Jobs/Chores referencing the Project — deleting it here does
// not cascade, by design.
func (s *Service) DeleteProject(ctx context.Context, ownerUserID, projectID string) error {
	return s.projects.DeleteProject(ctx, ownerUserID, projectID)
}

// AddAgentsFile creates a starter AGENTS.md in the Project's OutputDirectory
// if one doesn't already exist (FR-019/FR-021), enforcing ownership.
// Idempotent: calling this on a Project that already has an AGENTS.md is a
// no-op, not an error. Path resolution goes through fsguard.ResolveWithin
// rather than a direct filepath.Join, per package fsguard's mandate that
// every Job/Chore/Project filesystem operation use it.
//
// No locking is introduced around this write (spec.md Edge Case #8): the
// agent (via chat, not built by this Service) and the user (via direct
// filesystem access, FR-022) may both write AGENTS.md, and last write wins,
// consistent with normal filesystem semantics for a file also open in an
// external editor.
func (s *Service) AddAgentsFile(ctx context.Context, ownerUserID, projectID string) error {
	p, err := s.projects.GetProject(ctx, ownerUserID, projectID)
	if err != nil {
		return err
	}

	path, err := fsguard.ResolveWithin(p.OutputDirectory, "AGENTS.md")
	if err != nil {
		return fmt.Errorf("failed to resolve AGENTS.md path: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat AGENTS.md: %w", err)
	}

	const starter = "# AGENTS.md\n\nInstructions for the agent working in this Project.\n"
	if err := os.WriteFile(path, []byte(starter), 0644); err != nil {
		return fmt.Errorf("failed to create AGENTS.md: %w", err)
	}

	p.UpdatedAt = s.now()
	if err := s.projects.SaveProject(ctx, p); err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}

// SweepExpired deletes every Project owned by ownerUserID that is expired
// under policy (FR-023), measured from each Project's UpdatedAt, returning
// the count deleted. A "Never" policy deletes nothing.
func (s *Service) SweepExpired(ctx context.Context, ownerUserID string, policy domain.RetentionPolicy, now time.Time) (int, error) {
	if policy.IsNever() {
		return 0, nil
	}
	list, err := s.projects.ListProjects(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, p := range list {
		if policy.IsExpired(p.UpdatedAt, now) {
			if err := s.projects.DeleteProject(ctx, ownerUserID, p.ID); err != nil {
				continue // Best-effort sweep; one failure must not abort the rest.
			}
			deleted++
		}
	}
	return deleted, nil
}
