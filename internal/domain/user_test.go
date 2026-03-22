package domain

import (
	"testing"
)

func TestRole_Level(t *testing.T) {
	tests := []struct {
		role Role
		want int
	}{
		{RoleGuest, 0},
		{RoleUser, 1},
		{RoleAdmin, 2},
		{Role("unknown"), -1},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.Level(); got != tt.want {
				t.Errorf("Level() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRole_HasPermission(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		required Role
		want     bool
	}{
		{
			name:     "admin has admin permission",
			role:     RoleAdmin,
			required: RoleAdmin,
			want:     true,
		},
		{
			name:     "admin has user permission",
			role:     RoleAdmin,
			required: RoleUser,
			want:     true,
		},
		{
			name:     "admin has guest permission",
			role:     RoleAdmin,
			required: RoleGuest,
			want:     true,
		},
		{
			name:     "user has user permission",
			role:     RoleUser,
			required: RoleUser,
			want:     true,
		},
		{
			name:     "user has guest permission",
			role:     RoleUser,
			required: RoleGuest,
			want:     true,
		},
		{
			name:     "user does not have admin permission",
			role:     RoleUser,
			required: RoleAdmin,
			want:     false,
		},
		{
			name:     "guest has guest permission",
			role:     RoleGuest,
			required: RoleGuest,
			want:     true,
		},
		{
			name:     "guest does not have user permission",
			role:     RoleGuest,
			required: RoleUser,
			want:     false,
		},
		{
			name:     "guest does not have admin permission",
			role:     RoleGuest,
			required: RoleAdmin,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.HasPermission(tt.required); got != tt.want {
				t.Errorf("HasPermission(%v) = %v, want %v", tt.required, got, tt.want)
			}
		})
	}
}
