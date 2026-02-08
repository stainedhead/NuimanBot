package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nuimanbot/internal/domain"
)

// MockProfileService is a mock implementation for testing
type MockProfileService struct {
	CreateProfileFunc          func(ctx context.Context, profile *domain.UserProfile) error
	GetProfileFunc             func(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetProfileByEmailFunc      func(ctx context.Context, email string) (*domain.UserProfile, error)
	UpdateProfileFunc          func(ctx context.Context, userID string, updates map[string]interface{}) error
	DeleteProfileFunc          func(ctx context.Context, userID string) error
	ListProfilesFunc           func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
	GetProfileByPlatformIDFunc func(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error)
}

func (m *MockProfileService) CreateProfile(ctx context.Context, profile *domain.UserProfile) error {
	if m.CreateProfileFunc != nil {
		return m.CreateProfileFunc(ctx, profile)
	}
	return nil
}

func (m *MockProfileService) GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	if m.GetProfileFunc != nil {
		return m.GetProfileFunc(ctx, userID)
	}
	return nil, errors.New("not found")
}

func (m *MockProfileService) GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	if m.GetProfileByEmailFunc != nil {
		return m.GetProfileByEmailFunc(ctx, email)
	}
	return nil, errors.New("not found")
}

func (m *MockProfileService) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	if m.UpdateProfileFunc != nil {
		return m.UpdateProfileFunc(ctx, userID, updates)
	}
	return nil
}

func (m *MockProfileService) DeleteProfile(ctx context.Context, userID string) error {
	if m.DeleteProfileFunc != nil {
		return m.DeleteProfileFunc(ctx, userID)
	}
	return nil
}

func (m *MockProfileService) ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
	if m.ListProfilesFunc != nil {
		return m.ListProfilesFunc(ctx, offset, limit)
	}
	return []*domain.UserProfile{}, nil
}

func (m *MockProfileService) GetProfileByPlatformID(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error) {
	if m.GetProfileByPlatformIDFunc != nil {
		return m.GetProfileByPlatformIDFunc(ctx, platform, platformID)
	}
	return nil, errors.New("not found")
}

// TestProfileHandler_List tests listing profiles with pagination
func TestProfileHandler_List(t *testing.T) {
	profiles := []*domain.UserProfile{
		{UserID: "user-1", PrimaryEmail: "user1@example.com", Enabled: true},
		{UserID: "user-2", PrimaryEmail: "user2@example.com", Enabled: true},
	}

	mockService := &MockProfileService{
		ListProfilesFunc: func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
			assert.Equal(t, 0, offset)
			assert.Equal(t, 50, limit)
			return profiles, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ListProfilesResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 2, len(response.Profiles))
	assert.Equal(t, "user-1", response.Profiles[0].UserID)
}

// TestProfileHandler_Get tests getting a single profile
func TestProfileHandler_Get(t *testing.T) {
	profile := &domain.UserProfile{
		UserID:       "user-123",
		PrimaryEmail: "test@example.com",
		Role:         domain.RoleUser,
		Enabled:      true,
	}

	mockService := &MockProfileService{
		GetProfileFunc: func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			assert.Equal(t, "user-123", userID)
			return profile, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/admin/profiles/user-123", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response domain.UserProfile
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "user-123", response.UserID)
	assert.Equal(t, "test@example.com", response.PrimaryEmail)
}

// TestProfileHandler_Create tests creating a new profile
func TestProfileHandler_Create(t *testing.T) {
	mockService := &MockProfileService{
		CreateProfileFunc: func(ctx context.Context, profile *domain.UserProfile) error {
			assert.Equal(t, "new-user", profile.UserID)
			assert.Equal(t, "new@example.com", profile.PrimaryEmail)
			return nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	createReq := CreateProfileRequest{
		UserID:       "new-user",
		PrimaryEmail: "new@example.com",
		FirstName:    "New",
		LastName:     "User",
		Role:         domain.RoleUser,
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/v1/admin/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response domain.UserProfile
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "new-user", response.UserID)
}

// TestProfileHandler_Update tests updating a profile
func TestProfileHandler_Update(t *testing.T) {
	mockService := &MockProfileService{
		UpdateProfileFunc: func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "Pacific/Auckland", updates["timezone"])
			return nil
		},
		GetProfileFunc: func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			return &domain.UserProfile{
				UserID:       "user-123",
				PrimaryEmail: "test@example.com",
				Timezone:     "Pacific/Auckland",
				Enabled:      true,
			}, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	updateReq := UpdateProfileRequest{
		Timezone: stringPtr("Pacific/Auckland"),
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest("PUT", "/api/v1/admin/profiles/user-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response UpdateProfileResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Contains(t, response.UpdatedFields, "timezone")
	assert.Equal(t, "user-123", response.Profile.UserID)
}

// TestProfileHandler_Delete tests deleting a profile
func TestProfileHandler_Delete(t *testing.T) {
	mockService := &MockProfileService{
		DeleteProfileFunc: func(ctx context.Context, userID string) error {
			assert.Equal(t, "user-123", userID)
			return nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("DELETE", "/api/v1/admin/profiles/user-123", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// TestProfileHandler_ListWithPagination tests pagination parameters
func TestProfileHandler_ListWithPagination(t *testing.T) {
	mockService := &MockProfileService{
		ListProfilesFunc: func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
			assert.Equal(t, 10, offset)
			assert.Equal(t, 20, limit)
			return []*domain.UserProfile{}, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/admin/profiles?offset=10&limit=20", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
