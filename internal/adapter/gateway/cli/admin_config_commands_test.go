package cli

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// mockConfigReloader mocks the ConfigReloader interface for testing.
type mockConfigReloader struct {
	reloadFunc func(ctx context.Context) error
}

func (m *mockConfigReloader) Reload(ctx context.Context) error {
	if m.reloadFunc != nil {
		return m.reloadFunc(ctx)
	}
	return nil
}

func TestIsConfigCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"/admin config reload", true},
		{"/admin config help", true},
		{"/admin config status", true},
		{"/admin config ", true},
		{"/admin config", false},
		{"/admin bot list", false},
		{"config reload", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsConfigCommand(tt.input)
			if result != tt.expected {
				t.Errorf("IsConfigCommand(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAdminConfigCommandHandler_HandleConfigCommand(t *testing.T) {
	adminUser := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	regularUser := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	t.Run("rejects non-admin users", func(t *testing.T) {
		handler := NewAdminConfigCommandHandler(&mockConfigReloader{})

		_, err := handler.HandleConfigCommand(context.Background(), regularUser, "/admin config reload")
		if !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Errorf("expected ErrInsufficientPermissions, got %v", err)
		}
	})

	t.Run("reload succeeds", func(t *testing.T) {
		reloadCalled := false
		reloader := &mockConfigReloader{
			reloadFunc: func(ctx context.Context) error {
				reloadCalled = true
				return nil
			},
		}
		handler := NewAdminConfigCommandHandler(reloader)

		result, err := handler.HandleConfigCommand(context.Background(), adminUser, "/admin config reload")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reloadCalled {
			t.Fatal("expected Reload to be called")
		}
		if result == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("reload failure returns error message", func(t *testing.T) {
		reloader := &mockConfigReloader{
			reloadFunc: func(ctx context.Context) error {
				return errors.New("file not found")
			},
		}
		handler := NewAdminConfigCommandHandler(reloader)

		result, err := handler.HandleConfigCommand(context.Background(), adminUser, "/admin config reload")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == "" {
			t.Fatal("expected non-empty result with error details")
		}
	})

	t.Run("help shows usage", func(t *testing.T) {
		handler := NewAdminConfigCommandHandler(&mockConfigReloader{})

		result, err := handler.HandleConfigCommand(context.Background(), adminUser, "/admin config help")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == "" {
			t.Fatal("expected non-empty help text")
		}
	})

	t.Run("no subcommand shows help", func(t *testing.T) {
		handler := NewAdminConfigCommandHandler(&mockConfigReloader{})

		result, err := handler.HandleConfigCommand(context.Background(), adminUser, "/admin config ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == "" {
			t.Fatal("expected help text for missing subcommand")
		}
	})

	t.Run("unknown subcommand", func(t *testing.T) {
		handler := NewAdminConfigCommandHandler(&mockConfigReloader{})

		result, err := handler.HandleConfigCommand(context.Background(), adminUser, "/admin config unknown")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == "" {
			t.Fatal("expected non-empty result for unknown subcommand")
		}
	})
}
