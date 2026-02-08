package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"nuimanbot/internal/domain"
)

// ProfileService defines the interface for profile management operations
type ProfileService interface {
	CreateProfile(ctx context.Context, profile *domain.UserProfile) error
	GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error)
	UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error
	DeleteProfile(ctx context.Context, userID string) error
	ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
	GetProfileByPlatformID(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error)
}

// ProfileHandler handles HTTP requests for user profile operations
type ProfileHandler struct {
	service ProfileService
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(service ProfileService) *ProfileHandler {
	return &ProfileHandler{
		service: service,
	}
}

// RegisterRoutes registers all profile routes
func (h *ProfileHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/admin/profiles", h.List).Methods("GET")
	router.HandleFunc("/api/v1/admin/profiles/{id}", h.Get).Methods("GET")
	router.HandleFunc("/api/v1/admin/profiles", h.Create).Methods("POST")
	router.HandleFunc("/api/v1/admin/profiles/{id}", h.Update).Methods("PUT")
	router.HandleFunc("/api/v1/admin/profiles/{id}", h.Delete).Methods("DELETE")
}

// ListProfilesResponse is the response for list profiles endpoint
type ListProfilesResponse struct {
	Profiles []*domain.UserProfile `json:"profiles"`
	Offset   int                   `json:"offset"`
	Limit    int                   `json:"limit"`
	Total    int                   `json:"total"`
}

// CreateProfileRequest is the request body for creating a profile
type CreateProfileRequest struct {
	UserID            string                      `json:"userID"`
	Moniker           string                      `json:"moniker,omitempty"`
	FirstName         string                      `json:"firstName,omitempty"`
	LastName          string                      `json:"lastName,omitempty"`
	NickName          string                      `json:"nickName,omitempty"`
	PrimaryEmail      string                      `json:"primaryEmail"`
	BackupEmail       string                      `json:"backupEmail,omitempty"`
	MobilePhone       string                      `json:"mobilePhone,omitempty"`
	PrimaryLanguage   string                      `json:"primaryLanguage,omitempty"`
	SecondaryLanguage string                      `json:"secondaryLanguage,omitempty"`
	Timezone          string                      `json:"timezone,omitempty"`
	PrimaryLocation   string                      `json:"primaryLocation,omitempty"`
	JobRole           string                      `json:"jobRole,omitempty"`
	UserType          domain.UserType             `json:"userType,omitempty"`
	Role              domain.Role                 `json:"role,omitempty"`
	ProfileInfo       string                      `json:"profileInfo,omitempty"`
	PlatformIDs       *domain.PlatformIdentifiers `json:"platformIDs,omitempty"`
}

// UpdateProfileRequest is the request body for updating a profile (partial updates)
type UpdateProfileRequest struct {
	Moniker           *string                     `json:"moniker,omitempty"`
	FirstName         *string                     `json:"firstName,omitempty"`
	LastName          *string                     `json:"lastName,omitempty"`
	NickName          *string                     `json:"nickName,omitempty"`
	BackupEmail       *string                     `json:"backupEmail,omitempty"`
	MobilePhone       *string                     `json:"mobilePhone,omitempty"`
	PrimaryLanguage   *string                     `json:"primaryLanguage,omitempty"`
	SecondaryLanguage *string                     `json:"secondaryLanguage,omitempty"`
	Timezone          *string                     `json:"timezone,omitempty"`
	PrimaryLocation   *string                     `json:"primaryLocation,omitempty"`
	JobRole           *string                     `json:"jobRole,omitempty"`
	UserType          *domain.UserType            `json:"userType,omitempty"`
	Role              *domain.Role                `json:"role,omitempty"`
	ProfileInfo       *string                     `json:"profileInfo,omitempty"`
	Enabled           *bool                       `json:"enabled,omitempty"`
	PlatformIDs       *domain.PlatformIdentifiers `json:"platformIDs,omitempty"`
}

// UpdateProfileResponse is the response for update profile endpoint
type UpdateProfileResponse struct {
	Profile       *domain.UserProfile `json:"profile"`
	UpdatedFields []string            `json:"updatedFields"`
}

// List handles GET /api/v1/admin/profiles
func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	offset := 0
	limit := 50

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil {
			offset = val
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 500 {
			limit = val
		}
	}

	// Get profiles
	profiles, err := h.service.ListProfiles(r.Context(), offset, limit)
	if err != nil {
		http.Error(w, "Failed to list profiles", http.StatusInternalServerError)
		return
	}

	// Build response
	response := ListProfilesResponse{
		Profiles: profiles,
		Offset:   offset,
		Limit:    limit,
		Total:    len(profiles),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Get handles GET /api/v1/admin/profiles/{id}
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	profile, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// Create handles POST /api/v1/admin/profiles
func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build profile from request
	profile := &domain.UserProfile{
		UserID:            req.UserID,
		Moniker:           req.Moniker,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		NickName:          req.NickName,
		PrimaryEmail:      req.PrimaryEmail,
		BackupEmail:       req.BackupEmail,
		MobilePhone:       req.MobilePhone,
		PrimaryLanguage:   req.PrimaryLanguage,
		SecondaryLanguage: req.SecondaryLanguage,
		Timezone:          req.Timezone,
		PrimaryLocation:   req.PrimaryLocation,
		JobRole:           req.JobRole,
		UserType:          req.UserType,
		Role:              req.Role,
		ProfileInfo:       req.ProfileInfo,
		Enabled:           true, // New profiles are enabled by default
	}

	// Set platform IDs if provided
	if req.PlatformIDs != nil {
		profile.PlatformIDs = *req.PlatformIDs
	}

	// Set defaults
	if profile.PrimaryLanguage == "" {
		profile.PrimaryLanguage = "en"
	}
	if profile.Timezone == "" {
		profile.Timezone = "UTC"
	}
	if profile.Role == "" {
		profile.Role = domain.RoleUser
	}
	if profile.UserType == "" {
		profile.UserType = domain.UserTypeIndividual
	}

	// Create profile
	if err := h.service.CreateProfile(r.Context(), profile); err != nil {
		http.Error(w, "Failed to create profile: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(profile)
}

// Update handles PUT /api/v1/admin/profiles/{id}
func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build updates map from request (only include non-nil fields)
	updates := make(map[string]interface{})
	updatedFields := []string{}

	if req.Moniker != nil {
		updates["moniker"] = *req.Moniker
		updatedFields = append(updatedFields, "moniker")
	}
	if req.FirstName != nil {
		updates["firstName"] = *req.FirstName
		updatedFields = append(updatedFields, "firstName")
	}
	if req.LastName != nil {
		updates["lastName"] = *req.LastName
		updatedFields = append(updatedFields, "lastName")
	}
	if req.NickName != nil {
		updates["nickName"] = *req.NickName
		updatedFields = append(updatedFields, "nickName")
	}
	if req.BackupEmail != nil {
		updates["backupEmail"] = *req.BackupEmail
		updatedFields = append(updatedFields, "backupEmail")
	}
	if req.MobilePhone != nil {
		updates["mobilePhone"] = *req.MobilePhone
		updatedFields = append(updatedFields, "mobilePhone")
	}
	if req.PrimaryLanguage != nil {
		updates["primaryLanguage"] = *req.PrimaryLanguage
		updatedFields = append(updatedFields, "primaryLanguage")
	}
	if req.SecondaryLanguage != nil {
		updates["secondaryLanguage"] = *req.SecondaryLanguage
		updatedFields = append(updatedFields, "secondaryLanguage")
	}
	if req.Timezone != nil {
		updates["timezone"] = *req.Timezone
		updatedFields = append(updatedFields, "timezone")
	}
	if req.PrimaryLocation != nil {
		updates["primaryLocation"] = *req.PrimaryLocation
		updatedFields = append(updatedFields, "primaryLocation")
	}
	if req.JobRole != nil {
		updates["jobRole"] = *req.JobRole
		updatedFields = append(updatedFields, "jobRole")
	}
	if req.UserType != nil {
		updates["userType"] = *req.UserType
		updatedFields = append(updatedFields, "userType")
	}
	if req.Role != nil {
		updates["role"] = *req.Role
		updatedFields = append(updatedFields, "role")
	}
	if req.ProfileInfo != nil {
		updates["profileInfo"] = *req.ProfileInfo
		updatedFields = append(updatedFields, "profileInfo")
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
		updatedFields = append(updatedFields, "enabled")
	}
	if req.PlatformIDs != nil {
		updates["platformIDs"] = *req.PlatformIDs
		updatedFields = append(updatedFields, "platformIDs")
	}

	// Update profile
	if err := h.service.UpdateProfile(r.Context(), userID, updates); err != nil {
		http.Error(w, "Failed to update profile: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get updated profile
	profile, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to retrieve updated profile", http.StatusInternalServerError)
		return
	}

	response := UpdateProfileResponse{
		Profile:       profile,
		UpdatedFields: updatedFields,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles DELETE /api/v1/admin/profiles/{id}
func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	if err := h.service.DeleteProfile(r.Context(), userID); err != nil {
		http.Error(w, "Failed to delete profile", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
