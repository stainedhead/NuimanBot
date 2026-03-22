package domain

import (
	"strings"
	"testing"
)

func TestNewUserProfile(t *testing.T) {
	profile := NewUserProfile("user-1", "alice@example.com", UserTypeIndividual)

	if profile == nil {
		t.Fatal("NewUserProfile returned nil")
	}
	if profile.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", profile.UserID, "user-1")
	}
	if profile.PrimaryEmail != "alice@example.com" {
		t.Errorf("PrimaryEmail = %q, want %q", profile.PrimaryEmail, "alice@example.com")
	}
	if profile.UserType != UserTypeIndividual {
		t.Errorf("UserType = %q, want %q", profile.UserType, UserTypeIndividual)
	}
	if profile.Role != RoleUser {
		t.Errorf("Role = %q, want %q", profile.Role, RoleUser)
	}
	if profile.PrimaryLanguage != "en" {
		t.Errorf("PrimaryLanguage = %q, want %q", profile.PrimaryLanguage, "en")
	}
	if profile.Timezone != "UTC" {
		t.Errorf("Timezone = %q, want %q", profile.Timezone, "UTC")
	}
	if !profile.Enabled {
		t.Error("Enabled should default to true")
	}
	if profile.DataDirectory == "" {
		t.Error("DataDirectory should be set")
	}
	if profile.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if profile.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestUserProfile_Validate_AdditionalCases(t *testing.T) {
	baseProfile := func() *UserProfile {
		return &UserProfile{
			UserID:        "user-1",
			PrimaryEmail:  "alice@example.com",
			DataDirectory: "users/user-1",
		}
	}

	t.Run("userID too long", func(t *testing.T) {
		p := baseProfile()
		p.UserID = strings.Repeat("a", 65)
		if err := p.Validate(); err == nil {
			t.Error("expected error for userID > 64 chars")
		}
	})

	t.Run("firstName too long", func(t *testing.T) {
		p := baseProfile()
		p.FirstName = strings.Repeat("a", 101)
		if err := p.Validate(); err == nil {
			t.Error("expected error for firstName > 100 chars")
		}
	})

	t.Run("lastName too long", func(t *testing.T) {
		p := baseProfile()
		p.LastName = strings.Repeat("a", 101)
		if err := p.Validate(); err == nil {
			t.Error("expected error for lastName > 100 chars")
		}
	})

	t.Run("nickName too long", func(t *testing.T) {
		p := baseProfile()
		p.NickName = strings.Repeat("a", 51)
		if err := p.Validate(); err == nil {
			t.Error("expected error for nickName > 50 chars")
		}
	})

	t.Run("primaryEmail too long", func(t *testing.T) {
		p := baseProfile()
		// Build a valid email that is > 254 chars total
		// localpart@domain format, 255 chars total
		localPart := strings.Repeat("a", 200)
		p.PrimaryEmail = localPart + "@" + strings.Repeat("b", 50) + ".com" // > 254 chars
		if err := p.Validate(); err == nil {
			t.Error("expected error for primaryEmail > 254 chars")
		}
	})

	t.Run("valid backup email", func(t *testing.T) {
		p := baseProfile()
		p.BackupEmail = "backup@example.com"
		if err := p.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid backup email", func(t *testing.T) {
		p := baseProfile()
		p.BackupEmail = "not-an-email"
		if err := p.Validate(); err == nil {
			t.Error("expected error for invalid backup email")
		}
	})

	t.Run("backup email same as primary", func(t *testing.T) {
		p := baseProfile()
		p.BackupEmail = p.PrimaryEmail
		if err := p.Validate(); err == nil {
			t.Error("expected error when backup email equals primary email")
		}
	})

	t.Run("backup email too long", func(t *testing.T) {
		p := baseProfile()
		localPart := strings.Repeat("a", 200)
		p.BackupEmail = localPart + "@" + strings.Repeat("b", 50) + ".com" // > 254 chars
		if err := p.Validate(); err == nil {
			t.Error("expected error for backupEmail > 254 chars")
		}
	})

	t.Run("valid E.164 phone", func(t *testing.T) {
		p := baseProfile()
		p.MobilePhone = "+15551234567"
		if err := p.Validate(); err != nil {
			t.Errorf("unexpected error for valid phone: %v", err)
		}
	})

	t.Run("phone without plus prefix", func(t *testing.T) {
		p := baseProfile()
		p.MobilePhone = "15551234567"
		if err := p.Validate(); err == nil {
			t.Error("expected error for phone without + prefix")
		}
	})

	t.Run("phone too short", func(t *testing.T) {
		p := baseProfile()
		p.MobilePhone = "+12345" // only 6 chars
		if err := p.Validate(); err == nil {
			t.Error("expected error for phone < 8 chars")
		}
	})

	t.Run("phone too long", func(t *testing.T) {
		p := baseProfile()
		p.MobilePhone = "+123456789012345678" // 19 chars
		if err := p.Validate(); err == nil {
			t.Error("expected error for phone > 16 chars")
		}
	})

	t.Run("primaryLocation too long", func(t *testing.T) {
		p := baseProfile()
		p.PrimaryLocation = strings.Repeat("a", 101)
		if err := p.Validate(); err == nil {
			t.Error("expected error for primaryLocation > 100 chars")
		}
	})

	t.Run("jobRole too long", func(t *testing.T) {
		p := baseProfile()
		p.JobRole = strings.Repeat("a", 101)
		if err := p.Validate(); err == nil {
			t.Error("expected error for jobRole > 100 chars")
		}
	})

	t.Run("missing dataDirectory", func(t *testing.T) {
		p := baseProfile()
		p.DataDirectory = ""
		if err := p.Validate(); err == nil {
			t.Error("expected error for missing dataDirectory")
		}
	})
}
