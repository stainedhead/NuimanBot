package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nuimanbot/internal/domain"
)

// MockProfileService is a mock implementation of profile service for testing
type MockProfileService struct {
	GetByAPIKeyFunc func(ctx context.Context, apiKey string) (*domain.UserProfile, error)
}

func (m *MockProfileService) GetByAPIKey(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
	if m.GetByAPIKeyFunc != nil {
		return m.GetByAPIKeyFunc(ctx, apiKey)
	}
	return nil, assert.AnError
}

// TestAuthMiddleware_ValidToken tests authentication with a valid token
func TestAuthMiddleware_ValidToken(t *testing.T) {
	// Red: Write failing test first
	profile := &domain.UserProfile{
		UserID:       "user-123",
		PrimaryEmail: "test@example.com",
		Role:         domain.RoleUser,
		APIKey:       "valid-api-key-12345",
		Enabled:      true, // User is enabled
	}

	mockService := &MockProfileService{
		GetByAPIKeyFunc: func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
			if apiKey == "valid-api-key-12345" {
				return profile, nil
			}
			return nil, assert.AnError
		},
	}

	middleware := NewAuthMiddleware(mockService)

	// Create a test handler that verifies user is in context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(UserContextKey)
		require.NotNil(t, user, "User should be in context")

		userProfile, ok := user.(*domain.UserProfile)
		require.True(t, ok, "User should be a *domain.UserProfile")
		assert.Equal(t, "user-123", userProfile.UserID)
		assert.Equal(t, "test@example.com", userProfile.PrimaryEmail)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap handler with auth middleware
	handler := middleware.Authenticate(testHandler)

	// Create request with valid Bearer token
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer valid-api-key-12345")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestAuthMiddleware_MissingToken tests authentication with no token
func TestAuthMiddleware_MissingToken(t *testing.T) {
	mockService := &MockProfileService{}
	middleware := NewAuthMiddleware(mockService)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	handler := middleware.Authenticate(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	// No Authorization header

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestAuthMiddleware_InvalidToken tests authentication with an invalid token
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mockService := &MockProfileService{
		GetByAPIKeyFunc: func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
			return nil, assert.AnError
		},
	}

	middleware := NewAuthMiddleware(mockService)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	handler := middleware.Authenticate(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestAuthMiddleware_MalformedHeader tests authentication with malformed header
func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	mockService := &MockProfileService{}
	middleware := NewAuthMiddleware(mockService)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called")
	})

	handler := middleware.Authenticate(testHandler)

	testCases := []struct {
		name   string
		header string
	}{
		{"no Bearer prefix", "just-a-token"},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"extra parts", "Bearer token extra"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.Header.Set("Authorization", tc.header)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// TestAuthMiddleware_DisabledUser tests authentication with a disabled user
func TestAuthMiddleware_DisabledUser(t *testing.T) {
	disabledProfile := &domain.UserProfile{
		UserID:       "user-456",
		PrimaryEmail: "disabled@example.com",
		Role:         domain.RoleUser,
		APIKey:       "disabled-user-key",
		Enabled:      false, // Disabled user
	}

	mockService := &MockProfileService{
		GetByAPIKeyFunc: func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
			if apiKey == "disabled-user-key" {
				return disabledProfile, nil
			}
			return nil, assert.AnError
		},
	}

	middleware := NewAuthMiddleware(mockService)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called for disabled user")
	})

	handler := middleware.Authenticate(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer disabled-user-key")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
