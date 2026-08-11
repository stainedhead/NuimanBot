package domain

import (
	"path/filepath"
	"time"
)

// Project is a durable, directory-scoped workspace the agent can read/write
// against on the user's behalf (FR-017–FR-023). Unlike a Chat, a Project has
// a real output directory the user retains direct filesystem access to.
type Project struct {
	// ID is the unique identifier (UUID).
	ID string
	// OwnerUserID is the owning user (FR-009/FR-010). Full isolation is
	// enforced at the repository layer, not just the UI.
	OwnerUserID string
	// Name is the user-supplied Project name.
	Name string
	// OutputDirectory is the Project's user-visible working directory
	// (FR-017), which may contain an AGENTS.md (FR-019).
	OutputDirectory string
	// HiddenDirectory holds agent-managed memory/context files, never shown
	// in the Project's file view (FR-018).
	HiddenDirectory string
	// Retention is this Project's independently configurable retention
	// policy (FR-023).
	Retention RetentionPolicy
	// CreatedAt is when the Project was created.
	CreatedAt time.Time
	// UpdatedAt is the Project's last-activity time, used for retention
	// evaluation (RetentionPolicy.IsExpired), not CreatedAt.
	UpdatedAt time.Time
}

// AgentsFilePath returns the path AGENTS.md would live at within this
// Project's OutputDirectory (FR-019/FR-021), regardless of whether the file
// currently exists.
func (p *Project) AgentsFilePath() string {
	if p.OutputDirectory == "" {
		return ""
	}
	return filepath.Join(p.OutputDirectory, "AGENTS.md")
}
