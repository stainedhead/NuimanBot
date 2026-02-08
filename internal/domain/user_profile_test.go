package domain

import (
	"strings"
	"testing"
	"time"
)

func TestUserProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile *UserProfile
		wantErr bool
	}{
		{
			name: "valid profile",
			profile: &UserProfile{
				UserID:          "550e8400-e29b-41d4-a716-446655440000",
				PrimaryEmail:    "alice@example.com",
				PrimaryLanguage: "en",
				Timezone:        "America/Los_Angeles",
				UserType:        UserTypeIndividual,
				Enabled:         true,
				DataDirectory:   "users/550e8400-e29b-41d4-a716-446655440000",
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing user ID",
			profile: &UserProfile{
				PrimaryEmail:    "alice@example.com",
				PrimaryLanguage: "en",
				Timezone:        "UTC",
				UserType:        UserTypeIndividual,
			},
			wantErr: true,
		},
		{
			name: "missing primary email",
			profile: &UserProfile{
				UserID:          "550e8400-e29b-41d4-a716-446655440000",
				PrimaryLanguage: "en",
				Timezone:        "UTC",
				UserType:        UserTypeIndividual,
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			profile: &UserProfile{
				UserID:          "550e8400-e29b-41d4-a716-446655440000",
				PrimaryEmail:    "not-an-email",
				PrimaryLanguage: "en",
				Timezone:        "UTC",
				UserType:        UserTypeIndividual,
			},
			wantErr: true,
		},
		{
			name: "moniker too long",
			profile: &UserProfile{
				UserID:          "550e8400-e29b-41d4-a716-446655440000",
				Moniker:         strings.Repeat("a", 51),
				PrimaryEmail:    "alice@example.com",
				PrimaryLanguage: "en",
				Timezone:        "UTC",
				UserType:        UserTypeIndividual,
			},
			wantErr: true,
		},
		{
			name: "profile info too long",
			profile: &UserProfile{
				UserID:          "550e8400-e29b-41d4-a716-446655440000",
				PrimaryEmail:    "alice@example.com",
				ProfileInfo:     strings.Repeat("a", 2001),
				PrimaryLanguage: "en",
				Timezone:        "UTC",
				UserType:        UserTypeIndividual,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("UserProfile.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserProfile_GetDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		profile *UserProfile
		want    string
	}{
		{
			name: "nickname takes priority",
			profile: &UserProfile{
				UserID:    "user-123",
				NickName:  "Ally",
				Moniker:   "alice_admin",
				FirstName: "Alice",
			},
			want: "Ally",
		},
		{
			name: "moniker when no nickname",
			profile: &UserProfile{
				UserID:    "user-123",
				Moniker:   "alice_admin",
				FirstName: "Alice",
			},
			want: "alice_admin",
		},
		{
			name: "first name when no nickname or moniker",
			profile: &UserProfile{
				UserID:    "user-123",
				FirstName: "Alice",
			},
			want: "Alice",
		},
		{
			name: "user ID fallback",
			profile: &UserProfile{
				UserID: "user-123",
			},
			want: "user-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.GetDisplayName()
			if got != tt.want {
				t.Errorf("GetDisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserProfile_GetFullName(t *testing.T) {
	tests := []struct {
		name    string
		profile *UserProfile
		want    string
	}{
		{
			name: "both first and last name",
			profile: &UserProfile{
				FirstName: "Alice",
				LastName:  "Anderson",
			},
			want: "Alice Anderson",
		},
		{
			name: "only first name",
			profile: &UserProfile{
				FirstName: "Alice",
			},
			want: "Alice",
		},
		{
			name: "only last name",
			profile: &UserProfile{
				LastName: "Anderson",
			},
			want: "Anderson",
		},
		{
			name:    "neither",
			profile: &UserProfile{},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.GetFullName()
			if got != tt.want {
				t.Errorf("GetFullName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserProfile_GetPreferredLanguage(t *testing.T) {
	tests := []struct {
		name    string
		profile *UserProfile
		want    string
	}{
		{
			name:    "set language",
			profile: &UserProfile{PrimaryLanguage: "es"},
			want:    "es",
		},
		{
			name:    "default to en",
			profile: &UserProfile{},
			want:    "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.GetPreferredLanguage()
			if got != tt.want {
				t.Errorf("GetPreferredLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserProfile_GetTimezone(t *testing.T) {
	tests := []struct {
		name    string
		profile *UserProfile
		want    string
	}{
		{
			name:    "set timezone",
			profile: &UserProfile{Timezone: "America/Los_Angeles"},
			want:    "America/Los_Angeles",
		},
		{
			name:    "default to UTC",
			profile: &UserProfile{},
			want:    "UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.profile.GetTimezone()
			if got != tt.want {
				t.Errorf("GetTimezone() = %v, want %v", got, tt.want)
			}
		})
	}
}
