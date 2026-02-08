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

// TestProfileHandler_LinkSlack tests linking Slack ID
func TestProfileHandler_LinkSlack(t *testing.T) {
	mockService := &MockProfileService{
		UpdateProfileFunc: func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "U12345678", updates["platformIDs.slack"])
			return nil
		},
		GetProfileFunc: func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			return &domain.UserProfile{
				UserID: userID,
				PlatformIDs: domain.PlatformIdentifiers{
					Slack: "U12345678",
				},
			}, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	body := `{"slackID": "U12345678"}`
	req := httptest.NewRequest("PUT", "/api/v1/admin/profiles/user-123/integrations/slack", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var profile domain.UserProfile
	err := json.NewDecoder(rec.Body).Decode(&profile)
	require.NoError(t, err)
	assert.Equal(t, "U12345678", profile.PlatformIDs.Slack)
}

// TestProfileHandler_UnlinkSlack tests unlinking Slack ID
func TestProfileHandler_UnlinkSlack(t *testing.T) {
	mockService := &MockProfileService{
		UpdateProfileFunc: func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "", updates["platformIDs.slack"])
			return nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("DELETE", "/api/v1/admin/profiles/user-123/integrations/slack", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// TestProfileHandler_LinkTelegram tests linking Telegram ID
func TestProfileHandler_LinkTelegram(t *testing.T) {
	mockService := &MockProfileService{
		UpdateProfileFunc: func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "123456789", updates["platformIDs.telegram"])
			return nil
		},
		GetProfileFunc: func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			return &domain.UserProfile{
				UserID: userID,
				PlatformIDs: domain.PlatformIdentifiers{
					Telegram: "123456789",
				},
			}, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	body := `{"telegramID": "123456789"}`
	req := httptest.NewRequest("PUT", "/api/v1/admin/profiles/user-123/integrations/telegram", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var profile domain.UserProfile
	err := json.NewDecoder(rec.Body).Decode(&profile)
	require.NoError(t, err)
	assert.Equal(t, "123456789", profile.PlatformIDs.Telegram)
}

// TestProfileHandler_UnlinkTelegram tests unlinking Telegram ID
func TestProfileHandler_UnlinkTelegram(t *testing.T) {
	mockService := &MockProfileService{
		UpdateProfileFunc: func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "", updates["platformIDs.telegram"])
			return nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("DELETE", "/api/v1/admin/profiles/user-123/integrations/telegram", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// TestProfileHandler_Search tests searching profiles
func TestProfileHandler_Search(t *testing.T) {
	handler := NewProfileHandler(&MockProfileService{})
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/admin/profiles/search?q=engineer&fields=jobRole,profileInfo", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response SearchProfilesResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "engineer", response.Query)
	assert.Equal(t, []string{"jobRole", "profileInfo"}, response.Fields)
}

// TestProfileHandler_Import tests bulk import
func TestProfileHandler_Import(t *testing.T) {
	importedCount := 0
	mockService := &MockProfileService{
		CreateProfileFunc: func(ctx context.Context, profile *domain.UserProfile) error {
			importedCount++
			if profile.UserID == "user-fail" {
				return errors.New("import failed")
			}
			return nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	body := `{
		"profiles": [
			{"userID": "user-1", "primaryEmail": "user1@example.com"},
			{"userID": "user-fail", "primaryEmail": "fail@example.com"},
			{"userID": "user-2", "primaryEmail": "user2@example.com"}
		]
	}`
	req := httptest.NewRequest("POST", "/api/v1/admin/profiles/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusPartialContent, rec.Code)

	var response ImportResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, 2, response.Imported)
	assert.Equal(t, 1, response.Failed)
	assert.Equal(t, 1, len(response.Errors))
}

// TestProfileHandler_ExportJSON tests export as JSON
func TestProfileHandler_ExportJSON(t *testing.T) {
	profiles := []*domain.UserProfile{
		{UserID: "user-1", PrimaryEmail: "user1@example.com"},
		{UserID: "user-2", PrimaryEmail: "user2@example.com"},
	}

	mockService := &MockProfileService{
		ListProfilesFunc: func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
			return profiles, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/admin/profiles/export?format=json", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "profiles.json")

	var exported []*domain.UserProfile
	err := json.NewDecoder(rec.Body).Decode(&exported)
	require.NoError(t, err)
	assert.Equal(t, 2, len(exported))
}

// TestProfileHandler_ExportCSV tests export as CSV
func TestProfileHandler_ExportCSV(t *testing.T) {
	profiles := []*domain.UserProfile{
		{UserID: "user-1", Moniker: "john", FirstName: "John", LastName: "Doe",
			PrimaryEmail: "john@example.com", Role: domain.RoleUser, UserType: domain.UserTypeIndividual, Enabled: true},
	}

	mockService := &MockProfileService{
		ListProfilesFunc: func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
			return profiles, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/v1/admin/profiles/export?format=csv", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/csv", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "profiles.csv")

	csv := rec.Body.String()
	assert.Contains(t, csv, "userID,moniker,firstName")
	assert.Contains(t, csv, "user-1,john,John,Doe")
}

// TestProfileHandler_GetOwnProfile tests user getting their own profile
func TestProfileHandler_GetOwnProfile(t *testing.T) {
	profile := &domain.UserProfile{
		UserID:       "user-123",
		PrimaryEmail: "test@example.com",
		Role:         domain.RoleUser,
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

	req := httptest.NewRequest("GET", "/api/v1/profile", nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", "user-123"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response domain.UserProfile
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "user-123", response.UserID)
}

// TestProfileHandler_UpdateOwnProfile tests user updating their own profile
func TestProfileHandler_UpdateOwnProfile(t *testing.T) {
	mockService := &MockProfileService{
		UpdateProfileFunc: func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, "user-123", userID)
			assert.Equal(t, "Jane", updates["firstName"])
			// Ensure role and userType are not in updates
			_, hasRole := updates["role"]
			_, hasUserType := updates["userType"]
			assert.False(t, hasRole)
			assert.False(t, hasUserType)
			return nil
		},
		GetProfileFunc: func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			return &domain.UserProfile{
				UserID:    userID,
				FirstName: "Jane",
			}, nil
		},
	}

	handler := NewProfileHandler(mockService)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	body := `{"firstName": "Jane"}`
	req := httptest.NewRequest("PUT", "/api/v1/profile", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "userID", "user-123"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response UpdateProfileResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "Jane", response.Profile.FirstName)
}

// TestProfileHandler_UpdateOwnProfile_ForbiddenFields tests that users cannot update admin-only fields
func TestProfileHandler_UpdateOwnProfile_ForbiddenFields(t *testing.T) {
	handler := NewProfileHandler(&MockProfileService{})
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	body := `{"firstName": "Jane", "role": "admin"}`
	req := httptest.NewRequest("PUT", "/api/v1/profile", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "userID", "user-123"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
