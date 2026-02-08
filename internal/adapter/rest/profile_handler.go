package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
	// Bulk operations (must be before {id} route to avoid conflict)
	router.HandleFunc("/api/v1/admin/profiles/import", h.Import).Methods("POST")
	router.HandleFunc("/api/v1/admin/profiles/export", h.Export).Methods("GET")
	router.HandleFunc("/api/v1/admin/profiles/search", h.Search).Methods("GET")

	// CRUD operations
	router.HandleFunc("/api/v1/admin/profiles", h.List).Methods("GET")
	router.HandleFunc("/api/v1/admin/profiles", h.Create).Methods("POST")
	router.HandleFunc("/api/v1/admin/profiles/{id}", h.Get).Methods("GET")
	router.HandleFunc("/api/v1/admin/profiles/{id}", h.Update).Methods("PUT")
	router.HandleFunc("/api/v1/admin/profiles/{id}", h.Delete).Methods("DELETE")

	// Platform integration (more specific routes after CRUD)
	router.HandleFunc("/api/v1/admin/profiles/{id}/integrations/slack", h.LinkSlack).Methods("PUT")
	router.HandleFunc("/api/v1/admin/profiles/{id}/integrations/slack", h.UnlinkSlack).Methods("DELETE")
	router.HandleFunc("/api/v1/admin/profiles/{id}/integrations/telegram", h.LinkTelegram).Methods("PUT")
	router.HandleFunc("/api/v1/admin/profiles/{id}/integrations/telegram", h.UnlinkTelegram).Methods("DELETE")

	// User self-service (non-admin)
	router.HandleFunc("/api/v1/profile", h.GetOwnProfile).Methods("GET")
	router.HandleFunc("/api/v1/profile", h.UpdateOwnProfile).Methods("PUT")
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

// LinkSlackRequest is the request body for linking Slack ID
type LinkSlackRequest struct {
	SlackID string `json:"slackID"`
}

// LinkSlack handles PUT /api/v1/admin/profiles/{id}/integrations/slack
func (h *ProfileHandler) LinkSlack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	var req LinkSlackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SlackID == "" {
		http.Error(w, "slackID is required", http.StatusBadRequest)
		return
	}

	updates := map[string]interface{}{
		"platformIDs.slack": req.SlackID,
	}

	if err := h.service.UpdateProfile(r.Context(), userID, updates); err != nil {
		http.Error(w, "Failed to link Slack ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	profile, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to retrieve updated profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// UnlinkSlack handles DELETE /api/v1/admin/profiles/{id}/integrations/slack
func (h *ProfileHandler) UnlinkSlack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	updates := map[string]interface{}{
		"platformIDs.slack": "",
	}

	if err := h.service.UpdateProfile(r.Context(), userID, updates); err != nil {
		http.Error(w, "Failed to unlink Slack ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// LinkTelegramRequest is the request body for linking Telegram ID
type LinkTelegramRequest struct {
	TelegramID string `json:"telegramID"`
}

// LinkTelegram handles PUT /api/v1/admin/profiles/{id}/integrations/telegram
func (h *ProfileHandler) LinkTelegram(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	var req LinkTelegramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TelegramID == "" {
		http.Error(w, "telegramID is required", http.StatusBadRequest)
		return
	}

	updates := map[string]interface{}{
		"platformIDs.telegram": req.TelegramID,
	}

	if err := h.service.UpdateProfile(r.Context(), userID, updates); err != nil {
		http.Error(w, "Failed to link Telegram ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	profile, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to retrieve updated profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// UnlinkTelegram handles DELETE /api/v1/admin/profiles/{id}/integrations/telegram
func (h *ProfileHandler) UnlinkTelegram(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	updates := map[string]interface{}{
		"platformIDs.telegram": "",
	}

	if err := h.service.UpdateProfile(r.Context(), userID, updates); err != nil {
		http.Error(w, "Failed to unlink Telegram ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SearchProfilesResponse is the response for search profiles endpoint
type SearchProfilesResponse struct {
	Profiles []*domain.UserProfile `json:"profiles"`
	Query    string                `json:"query"`
	Fields   []string              `json:"fields"`
	Total    int                   `json:"total"`
}

// Search handles GET /api/v1/admin/profiles/search
func (h *ProfileHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	fieldsStr := r.URL.Query().Get("fields")
	var fields []string
	if fieldsStr != "" {
		fields = strings.Split(fieldsStr, ",")
	}

	// TODO: Implement search in service layer
	// For now, return empty results
	response := SearchProfilesResponse{
		Profiles: []*domain.UserProfile{},
		Query:    query,
		Fields:   fields,
		Total:    0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ImportRequest is the request body for bulk import
type ImportRequest struct {
	Profiles []*domain.UserProfile `json:"profiles"`
}

// ImportResponse is the response for bulk import
type ImportResponse struct {
	Imported int      `json:"imported"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// Import handles POST /api/v1/admin/profiles/import
func (h *ProfileHandler) Import(w http.ResponseWriter, r *http.Request) {
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	imported := 0
	failed := 0
	errors := []string{}

	for _, profile := range req.Profiles {
		if err := h.service.CreateProfile(r.Context(), profile); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("Failed to import %s: %v", profile.UserID, err))
		} else {
			imported++
		}
	}

	response := ImportResponse{
		Imported: imported,
		Failed:   failed,
		Errors:   errors,
	}

	w.Header().Set("Content-Type", "application/json")
	if failed > 0 {
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(response)
}

// Export handles GET /api/v1/admin/profiles/export
func (h *ProfileHandler) Export(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// Get all profiles
	profiles, err := h.service.ListProfiles(r.Context(), 0, 10000)
	if err != nil {
		http.Error(w, "Failed to export profiles", http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=profiles.json")
		json.NewEncoder(w).Encode(profiles)
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=profiles.csv")

		// Write CSV header
		fmt.Fprintf(w, "userID,moniker,firstName,lastName,primaryEmail,role,userType,enabled\n")

		// Write CSV rows
		for _, p := range profiles {
			fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%s,%v\n",
				p.UserID, p.Moniker, p.FirstName, p.LastName,
				p.PrimaryEmail, p.Role, p.UserType, p.Enabled)
		}
	default:
		http.Error(w, "Invalid format. Supported: json, csv", http.StatusBadRequest)
	}
}

// GetOwnProfile handles GET /api/v1/profile (user self-service)
func (h *ProfileHandler) GetOwnProfile(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "User ID not found in request context", http.StatusUnauthorized)
		return
	}

	profile, err := h.service.GetProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// UpdateOwnProfile handles PUT /api/v1/profile (user self-service)
func (h *ProfileHandler) UpdateOwnProfile(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "User ID not found in request context", http.StatusUnauthorized)
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Users cannot change their own role or userType (admin-only fields)
	if req.Role != nil || req.UserType != nil {
		http.Error(w, "Cannot update role or userType via self-service endpoint", http.StatusForbidden)
		return
	}

	// Build updates map (reuse logic from Update handler)
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
	if req.ProfileInfo != nil {
		updates["profileInfo"] = *req.ProfileInfo
		updatedFields = append(updatedFields, "profileInfo")
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
