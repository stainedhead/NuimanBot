package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"nuimanbot/internal/domain"
)

// TestRequireRole_AdminRole tests that admin role can access admin endpoints
func TestRequireRole_AdminRole(t *testing.T) {
	adminProfile := &domain.UserProfile{
		UserID: "admin-123",
		Role:   domain.RoleAdmin,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Apply RequireRole middleware for admin role
	handler := RequireRole(domain.RoleAdmin)(testHandler)

	req := httptest.NewRequest("GET", "/api/admin/test", nil)
	req = req.WithContext(WithUser(req.Context(), adminProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestRequireRole_UserRoleBlocked tests that user role cannot access admin endpoints
func TestRequireRole_UserRoleBlocked(t *testing.T) {
	userProfile := &domain.UserProfile{
		UserID: "user-123",
		Role:   domain.RoleUser,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called for user role")
	})

	// Apply RequireRole middleware for admin role
	handler := RequireRole(domain.RoleAdmin)(testHandler)

	req := httptest.NewRequest("GET", "/api/admin/test", nil)
	req = req.WithContext(WithUser(req.Context(), userProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRequireRole_GuestRoleBlocked tests that guest role cannot access user endpoints
func TestRequireRole_GuestRoleBlocked(t *testing.T) {
	guestProfile := &domain.UserProfile{
		UserID: "guest-123",
		Role:   domain.RoleGuest,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called for guest role")
	})

	// Apply RequireRole middleware for user role
	handler := RequireRole(domain.RoleUser)(testHandler)

	req := httptest.NewRequest("GET", "/api/user/test", nil)
	req = req.WithContext(WithUser(req.Context(), guestProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRequireRole_MultipleRoles tests that multiple roles can be allowed
func TestRequireRole_MultipleRoles(t *testing.T) {
	testCases := []struct {
		name     string
		userRole domain.Role
		allowed  []domain.Role
		expected int
	}{
		{
			name:     "admin allowed when both admin and user specified",
			userRole: domain.RoleAdmin,
			allowed:  []domain.Role{domain.RoleAdmin, domain.RoleUser},
			expected: http.StatusOK,
		},
		{
			name:     "user allowed when both admin and user specified",
			userRole: domain.RoleUser,
			allowed:  []domain.Role{domain.RoleAdmin, domain.RoleUser},
			expected: http.StatusOK,
		},
		{
			name:     "guest blocked when only admin and user specified",
			userRole: domain.RoleGuest,
			allowed:  []domain.Role{domain.RoleAdmin, domain.RoleUser},
			expected: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			profile := &domain.UserProfile{
				UserID: "test-user",
				Role:   tc.userRole,
			}

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			handler := RequireRole(tc.allowed...)(testHandler)

			req := httptest.NewRequest("GET", "/api/test", nil)
			req = req.WithContext(WithUser(req.Context(), profile))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.expected, rec.Code)
		})
	}
}

// TestRequireRole_NoUserInContext tests that request fails when no user in context
func TestRequireRole_NoUserInContext(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called when no user in context")
	})

	handler := RequireRole(domain.RoleUser)(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	// No user in context

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestRequireAdmin_AdminAllowed tests the RequireAdmin helper
func TestRequireAdmin_AdminAllowed(t *testing.T) {
	adminProfile := &domain.UserProfile{
		UserID: "admin-123",
		Role:   domain.RoleAdmin,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := RequireAdmin(testHandler)

	req := httptest.NewRequest("GET", "/api/admin/test", nil)
	req = req.WithContext(WithUser(req.Context(), adminProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestRequireAdmin_UserBlocked tests that RequireAdmin blocks non-admin users
func TestRequireAdmin_UserBlocked(t *testing.T) {
	userProfile := &domain.UserProfile{
		UserID: "user-123",
		Role:   domain.RoleUser,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called for user role")
	})

	handler := RequireAdmin(testHandler)

	req := httptest.NewRequest("GET", "/api/admin/test", nil)
	req = req.WithContext(WithUser(req.Context(), userProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRequireUser_UserAllowed tests the RequireUser helper
func TestRequireUser_UserAllowed(t *testing.T) {
	userProfile := &domain.UserProfile{
		UserID: "user-123",
		Role:   domain.RoleUser,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := RequireUser(testHandler)

	req := httptest.NewRequest("GET", "/api/user/test", nil)
	req = req.WithContext(WithUser(req.Context(), userProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestRequireUser_AdminAlsoAllowed tests that admin can access user endpoints
func TestRequireUser_AdminAlsoAllowed(t *testing.T) {
	adminProfile := &domain.UserProfile{
		UserID: "admin-123",
		Role:   domain.RoleAdmin,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := RequireUser(testHandler)

	req := httptest.NewRequest("GET", "/api/user/test", nil)
	req = req.WithContext(WithUser(req.Context(), adminProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

// TestRequireUser_GuestBlocked tests that guest cannot access user endpoints
func TestRequireUser_GuestBlocked(t *testing.T) {
	guestProfile := &domain.UserProfile{
		UserID: "guest-123",
		Role:   domain.RoleGuest,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called for guest role")
	})

	handler := RequireUser(testHandler)

	req := httptest.NewRequest("GET", "/api/user/test", nil)
	req = req.WithContext(WithUser(req.Context(), guestProfile))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
