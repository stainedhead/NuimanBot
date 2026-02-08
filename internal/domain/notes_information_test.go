package domain

import (
	"encoding/json"
	"testing"
	"time"
)

// TestNewNotesInformation tests the constructor returns valid defaults
func TestNewNotesInformation(t *testing.T) {
	notes := NewNotesInformation()

	// Verify default values
	if notes.AdminNotes == nil {
		t.Error("expected AdminNotes to be initialized, got nil")
	}
	if len(notes.AdminNotes) != 0 {
		t.Errorf("expected empty AdminNotes, got %d items", len(notes.AdminNotes))
	}
	if notes.Flags == nil {
		t.Error("expected Flags to be initialized, got nil")
	}
	if len(notes.Flags) != 0 {
		t.Errorf("expected empty Flags, got %d items", len(notes.Flags))
	}
	if notes.SupportTickets == nil {
		t.Error("expected SupportTickets to be initialized, got nil")
	}
	if len(notes.SupportTickets) != 0 {
		t.Errorf("expected empty SupportTickets, got %d items", len(notes.SupportTickets))
	}
	if notes.Restrictions.FeatureAccess == nil {
		t.Error("expected Restrictions.FeatureAccess to be initialized, got nil")
	}
	if notes.Restrictions.BlockedFeatures == nil {
		t.Error("expected Restrictions.BlockedFeatures to be initialized, got nil")
	}
	if notes.CustomMetadata == nil {
		t.Error("expected CustomMetadata to be initialized, got nil")
	}
}

// TestNotesInformationValidate_Valid tests validation passes for valid notes
func TestNotesInformationValidate_Valid(t *testing.T) {
	tests := []struct {
		name  string
		notes NotesInformation
	}{
		{
			name:  "default notes",
			notes: NewNotesInformation(),
		},
		{
			name: "with admin notes",
			notes: NotesInformation{
				AdminNotes: []AdminNote{
					{
						Timestamp: time.Now(),
						AuthorID:  "admin-001",
						Note:      "User requested beta access",
					},
				},
				Flags:          make(map[string]bool),
				SupportTickets: []string{},
				Restrictions: Restrictions{
					FeatureAccess:   []string{},
					BlockedFeatures: []string{},
				},
				CustomMetadata: make(map[string]any),
			},
		},
		{
			name: "with flags and restrictions",
			notes: NotesInformation{
				AdminNotes: []AdminNote{},
				Flags: map[string]bool{
					"beta_tester":  true,
					"early_access": false,
				},
				SupportTickets: []string{"TICKET-123", "TICKET-456"},
				Restrictions: Restrictions{
					RateLimitOverride: intPtr(100),
					FeatureAccess:     []string{"bedrock", "advanced-tools"},
					BlockedFeatures:   []string{"experimental"},
				},
				CustomMetadata: map[string]any{
					"department": "Engineering",
				},
			},
		},
		{
			name: "max length note",
			notes: NotesInformation{
				AdminNotes: []AdminNote{
					{
						Timestamp: time.Now(),
						AuthorID:  "admin-001",
						Note:      string(make([]byte, 1000)), // Exactly 1000 chars
					},
				},
				Flags:          make(map[string]bool),
				SupportTickets: []string{},
				Restrictions:   Restrictions{FeatureAccess: []string{}, BlockedFeatures: []string{}},
				CustomMetadata: make(map[string]any),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.notes.Validate(); err != nil {
				t.Errorf("expected valid notes, got error: %v", err)
			}
		})
	}
}

// TestNotesInformationValidate_Invalid tests validation fails for invalid notes
func TestNotesInformationValidate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		notes   NotesInformation
		wantErr string
	}{
		{
			name: "missing admin note author",
			notes: NotesInformation{
				AdminNotes: []AdminNote{
					{
						Timestamp: time.Now(),
						AuthorID:  "", // Required
						Note:      "Test note",
					},
				},
				Flags:          make(map[string]bool),
				SupportTickets: []string{},
				Restrictions:   Restrictions{FeatureAccess: []string{}, BlockedFeatures: []string{}},
				CustomMetadata: make(map[string]any),
			},
			wantErr: "admin note author ID is required",
		},
		{
			name: "note too long",
			notes: NotesInformation{
				AdminNotes: []AdminNote{
					{
						Timestamp: time.Now(),
						AuthorID:  "admin-001",
						Note:      string(make([]byte, 1001)), // Over 1000 chars
					},
				},
				Flags:          make(map[string]bool),
				SupportTickets: []string{},
				Restrictions:   Restrictions{FeatureAccess: []string{}, BlockedFeatures: []string{}},
				CustomMetadata: make(map[string]any),
			},
			wantErr: "admin note exceeds maximum length of 1000 characters",
		},
		{
			name: "invalid rate limit",
			notes: NotesInformation{
				AdminNotes:     []AdminNote{},
				Flags:          make(map[string]bool),
				SupportTickets: []string{},
				Restrictions: Restrictions{
					RateLimitOverride: intPtr(0), // Must be > 0
					FeatureAccess:     []string{},
					BlockedFeatures:   []string{},
				},
				CustomMetadata: make(map[string]any),
			},
			wantErr: "rate limit override must be greater than 0",
		},
		{
			name: "negative rate limit",
			notes: NotesInformation{
				AdminNotes:     []AdminNote{},
				Flags:          make(map[string]bool),
				SupportTickets: []string{},
				Restrictions: Restrictions{
					RateLimitOverride: intPtr(-5),
					FeatureAccess:     []string{},
					BlockedFeatures:   []string{},
				},
				CustomMetadata: make(map[string]any),
			},
			wantErr: "rate limit override must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.notes.Validate()
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
			} else if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// TestNotesInformationAddAdminNote tests adding admin notes
func TestNotesInformationAddAdminNote(t *testing.T) {
	notes := NewNotesInformation()

	// Add first note
	notes.AddAdminNote("admin-001", "First note")
	if len(notes.AdminNotes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes.AdminNotes))
	}
	if notes.AdminNotes[0].AuthorID != "admin-001" {
		t.Errorf("expected AuthorID=admin-001, got %s", notes.AdminNotes[0].AuthorID)
	}
	if notes.AdminNotes[0].Note != "First note" {
		t.Errorf("expected Note='First note', got %s", notes.AdminNotes[0].Note)
	}

	// Add second note
	notes.AddAdminNote("admin-002", "Second note")
	if len(notes.AdminNotes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes.AdminNotes))
	}
	if notes.AdminNotes[1].AuthorID != "admin-002" {
		t.Errorf("expected AuthorID=admin-002, got %s", notes.AdminNotes[1].AuthorID)
	}
}

// TestNotesInformationGetLatestNote tests retrieving the latest note
func TestNotesInformationGetLatestNote(t *testing.T) {
	// Test empty notes
	notes := NewNotesInformation()
	if latest := notes.GetLatestNote(); latest != nil {
		t.Errorf("expected nil for empty notes, got %v", latest)
	}

	// Add notes
	notes.AddAdminNote("admin-001", "First note")
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	notes.AddAdminNote("admin-002", "Second note")
	time.Sleep(1 * time.Millisecond)
	notes.AddAdminNote("admin-003", "Third note")

	// Get latest
	latest := notes.GetLatestNote()
	if latest == nil {
		t.Fatal("expected latest note, got nil")
	}
	if latest.AuthorID != "admin-003" {
		t.Errorf("expected AuthorID=admin-003, got %s", latest.AuthorID)
	}
	if latest.Note != "Third note" {
		t.Errorf("expected Note='Third note', got %s", latest.Note)
	}
}

// TestNotesInformationSetFlag tests setting flags
func TestNotesInformationSetFlag(t *testing.T) {
	notes := NewNotesInformation()

	// Set flag to true
	notes.SetFlag("beta_tester", true)
	if !notes.Flags["beta_tester"] {
		t.Error("expected beta_tester=true")
	}

	// Set flag to false
	notes.SetFlag("beta_tester", false)
	if notes.Flags["beta_tester"] {
		t.Error("expected beta_tester=false")
	}

	// Set multiple flags
	notes.SetFlag("early_access", true)
	notes.SetFlag("premium", true)
	if len(notes.Flags) != 3 { // beta_tester, early_access, premium
		t.Errorf("expected 3 flags, got %d", len(notes.Flags))
	}
}

// TestNotesInformationAddFeatureAccess tests adding feature access
func TestNotesInformationAddFeatureAccess(t *testing.T) {
	notes := NewNotesInformation()

	// Add first feature
	notes.AddFeatureAccess("bedrock")
	if len(notes.Restrictions.FeatureAccess) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(notes.Restrictions.FeatureAccess))
	}
	if notes.Restrictions.FeatureAccess[0] != "bedrock" {
		t.Errorf("expected feature=bedrock, got %s", notes.Restrictions.FeatureAccess[0])
	}

	// Add second feature
	notes.AddFeatureAccess("advanced-tools")
	if len(notes.Restrictions.FeatureAccess) != 2 {
		t.Fatalf("expected 2 features, got %d", len(notes.Restrictions.FeatureAccess))
	}

	// Adding duplicate should not create duplicate entry
	notes.AddFeatureAccess("bedrock")
	if len(notes.Restrictions.FeatureAccess) != 2 {
		t.Errorf("expected 2 features (no duplicates), got %d", len(notes.Restrictions.FeatureAccess))
	}
}

// TestNotesInformationJSON tests JSON marshaling and unmarshaling
func TestNotesInformationJSON(t *testing.T) {
	original := NotesInformation{
		AdminNotes: []AdminNote{
			{
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				AuthorID:  "admin-001",
				Note:      "Test note",
			},
		},
		Flags: map[string]bool{
			"beta_tester":  true,
			"early_access": false,
		},
		SupportTickets: []string{"TICKET-123"},
		Restrictions: Restrictions{
			RateLimitOverride: intPtr(100),
			FeatureAccess:     []string{"bedrock"},
			BlockedFeatures:   []string{"experimental"},
		},
		CustomMetadata: map[string]any{
			"department": "Engineering",
			"priority":   5,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal from JSON
	var decoded NotesInformation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Compare fields
	if len(decoded.AdminNotes) != len(original.AdminNotes) {
		t.Errorf("AdminNotes length mismatch: got %d, want %d", len(decoded.AdminNotes), len(original.AdminNotes))
	}
	if len(decoded.Flags) != len(original.Flags) {
		t.Errorf("Flags length mismatch: got %d, want %d", len(decoded.Flags), len(original.Flags))
	}
	if decoded.Flags["beta_tester"] != original.Flags["beta_tester"] {
		t.Errorf("Flags[beta_tester] mismatch")
	}
	if len(decoded.SupportTickets) != len(original.SupportTickets) {
		t.Errorf("SupportTickets length mismatch")
	}
	if *decoded.Restrictions.RateLimitOverride != *original.Restrictions.RateLimitOverride {
		t.Errorf("RateLimitOverride mismatch")
	}
}

// TestNotesInformationJSON_NilValues tests JSON handling with nil values
func TestNotesInformationJSON_NilValues(t *testing.T) {
	original := NotesInformation{
		AdminNotes:     nil, // nil slices should be handled
		Flags:          nil, // nil maps should be handled
		SupportTickets: nil,
		Restrictions: Restrictions{
			RateLimitOverride: nil, // nil pointer is valid
			FeatureAccess:     nil,
			BlockedFeatures:   nil,
		},
		CustomMetadata: nil,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal from JSON
	var decoded NotesInformation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Should not panic when validating
	if err := decoded.Validate(); err != nil {
		t.Errorf("validation failed after unmarshal: %v", err)
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
