package domain

import (
	"errors"
	"time"
)

const (
	// MaxAdminNoteLength is the maximum length for admin note content.
	MaxAdminNoteLength = 1000
)

// AdminNote represents a timestamped note from an admin user.
// Notes are append-only and maintain a chronological audit trail.
type AdminNote struct {
	Timestamp time.Time `json:"timestamp"` // When note was created
	AuthorID  string    `json:"authorID"`  // Admin user ID who created note
	Note      string    `json:"note"`      // Note content (max MaxAdminNoteLength chars)
}

// Restrictions stores access control overrides and limits.
type Restrictions struct {
	RateLimitOverride *int     `json:"rateLimitOverride"` // Override rate limit (req/min), nil = use default
	FeatureAccess     []string `json:"featureAccess"`     // Allowed features (e.g., ["bedrock", "advanced-tools"])
	BlockedFeatures   []string `json:"blockedFeatures"`   // Explicitly blocked features
}

// NotesInformation stores admin notes, flags, and metadata for a user.
// Used for tracking support tickets, beta features, restrictions, etc.
// This is a value object that is part of UserProfile.
type NotesInformation struct {
	// Admin Notes
	AdminNotes []AdminNote `json:"adminNotes"` // Chronological list of admin notes

	// Feature Flags
	Flags map[string]bool `json:"flags"` // Feature flags (betaTester, earlyAccess, etc.)

	// Support Context
	SupportTickets []string `json:"supportTickets"` // Associated support ticket IDs

	// Access Restrictions
	Restrictions Restrictions `json:"restrictions"` // Rate limits, feature access overrides

	// Custom Metadata
	CustomMetadata map[string]any `json:"customMetadata"` // Freeform metadata
}

// NewNotesInformation creates a new NotesInformation with empty collections.
// All maps and slices are initialized to prevent nil pointer issues.
func NewNotesInformation() NotesInformation {
	return NotesInformation{
		AdminNotes:     []AdminNote{},
		Flags:          make(map[string]bool),
		SupportTickets: []string{},
		Restrictions: Restrictions{
			RateLimitOverride: nil,
			FeatureAccess:     []string{},
			BlockedFeatures:   []string{},
		},
		CustomMetadata: make(map[string]any),
	}
}

// Validate checks if the notes information is valid.
// Validates admin notes, rate limit overrides, and other constraints.
func (ni *NotesInformation) Validate() error {
	// Validate admin notes
	for _, note := range ni.AdminNotes {
		if note.AuthorID == "" {
			return errors.New("admin note author ID is required")
		}
		if len(note.Note) > MaxAdminNoteLength {
			return errors.New("admin note exceeds maximum length of 1000 characters")
		}
	}

	// Validate rate limit override
	if ni.Restrictions.RateLimitOverride != nil && *ni.Restrictions.RateLimitOverride <= 0 {
		return errors.New("rate limit override must be greater than 0")
	}

	return nil
}

// AddAdminNote adds a new admin note with the current timestamp.
func (ni *NotesInformation) AddAdminNote(authorID, note string) {
	adminNote := AdminNote{
		Timestamp: time.Now(),
		AuthorID:  authorID,
		Note:      note,
	}
	ni.AdminNotes = append(ni.AdminNotes, adminNote)
}

// GetLatestNote returns the most recent admin note, or nil if there are no notes.
func (ni *NotesInformation) GetLatestNote() *AdminNote {
	if len(ni.AdminNotes) == 0 {
		return nil
	}
	return &ni.AdminNotes[len(ni.AdminNotes)-1]
}

// SetFlag sets a feature flag to the specified value.
func (ni *NotesInformation) SetFlag(name string, value bool) {
	if ni.Flags == nil {
		ni.Flags = make(map[string]bool)
	}
	ni.Flags[name] = value
}

// AddFeatureAccess adds a feature to the allowed features list.
// Does not add duplicates if the feature is already present.
func (ni *NotesInformation) AddFeatureAccess(feature string) {
	// Check if feature already exists
	if ni.hasFeatureAccess(feature) {
		return
	}
	ni.Restrictions.FeatureAccess = append(ni.Restrictions.FeatureAccess, feature)
}

// hasFeatureAccess checks if a feature is in the allowed features list.
func (ni *NotesInformation) hasFeatureAccess(feature string) bool {
	for _, f := range ni.Restrictions.FeatureAccess {
		if f == feature {
			return true
		}
	}
	return false
}
