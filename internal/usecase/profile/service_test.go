package profile_test

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/profile"
)

// MockUserProfileRepository implements domain.UserProfileRepository for testing
type MockUserProfileRepository struct {
	SaveProfileFunc            func(ctx context.Context, p *domain.UserProfile) error
	GetProfileByUserIDFunc     func(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetProfileByEmailFunc      func(ctx context.Context, email string) (*domain.UserProfile, error)
	GetProfileByPlatformIDFunc func(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error)
	ListProfilesFunc           func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
	DeleteProfileFunc          func(ctx context.Context, userID string) error
	profiles                   map[string]*domain.UserProfile // In-memory store
}

func NewMockUserProfileRepository() *MockUserProfileRepository {
	return &MockUserProfileRepository{
		profiles: make(map[string]*domain.UserProfile),
	}
}

func (m *MockUserProfileRepository) SaveProfile(ctx context.Context, p *domain.UserProfile) error {
	if m.SaveProfileFunc != nil {
		return m.SaveProfileFunc(ctx, p)
	}
	m.profiles[p.UserID] = p
	return nil
}

func (m *MockUserProfileRepository) GetProfileByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	if m.GetProfileByUserIDFunc != nil {
		return m.GetProfileByUserIDFunc(ctx, userID)
	}
	profile, ok := m.profiles[userID]
	if !ok {
		return nil, errors.New("profile not found")
	}
	return profile, nil
}

func (m *MockUserProfileRepository) GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	if m.GetProfileByEmailFunc != nil {
		return m.GetProfileByEmailFunc(ctx, email)
	}
	for _, p := range m.profiles {
		if p.PrimaryEmail == email {
			return p, nil
		}
	}
	return nil, errors.New("profile not found")
}

func (m *MockUserProfileRepository) GetProfileByPlatformID(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error) {
	if m.GetProfileByPlatformIDFunc != nil {
		return m.GetProfileByPlatformIDFunc(ctx, platform, platformID)
	}
	for _, p := range m.profiles {
		switch platform {
		case domain.PlatformSlack:
			if p.PlatformIDs.Slack == platformID {
				return p, nil
			}
		case domain.PlatformTelegram:
			if p.PlatformIDs.Telegram == platformID {
				return p, nil
			}
		case domain.PlatformCLI:
			if p.PlatformIDs.CLI == platformID {
				return p, nil
			}
		}
	}
	return nil, errors.New("profile not found")
}

func (m *MockUserProfileRepository) ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
	if m.ListProfilesFunc != nil {
		return m.ListProfilesFunc(ctx, offset, limit)
	}
	profiles := make([]*domain.UserProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	// Apply pagination
	if offset >= len(profiles) {
		return []*domain.UserProfile{}, nil
	}
	end := offset + limit
	if end > len(profiles) {
		end = len(profiles)
	}
	return profiles[offset:end], nil
}

func (m *MockUserProfileRepository) DeleteProfile(ctx context.Context, userID string) error {
	if m.DeleteProfileFunc != nil {
		return m.DeleteProfileFunc(ctx, userID)
	}
	if _, ok := m.profiles[userID]; !ok {
		return errors.New("profile not found")
	}
	delete(m.profiles, userID)
	return nil
}

func (m *MockUserProfileRepository) GetProfileByAPIKey(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
	for _, p := range m.profiles {
		if p.APIKey == apiKey {
			return p, nil
		}
	}
	return nil, errors.New("profile not found")
}

// MockSecurityService implements domain.SecurityService for testing
type MockSecurityService struct {
	AuditFunc func(ctx context.Context, event *domain.AuditEvent) error
}

func (m *MockSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (m *MockSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (m *MockSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	if len(input) > maxLength {
		return "", errors.New("input exceeds max length")
	}
	return input, nil
}

func (m *MockSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if m.AuditFunc != nil {
		return m.AuditFunc(ctx, event)
	}
	return nil
}

func (m *MockSecurityService) GenerateAPIKey(ctx context.Context) (string, error) {
	return "mock-api-key-12345678", nil
}

// Test CreateProfile - success case
func TestCreateProfile_Success(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p.Moniker = "alice"
	p.FirstName = "Alice"
	p.LastName = "Smith"

	err := svc.CreateProfile(ctx, p)
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	// Verify profile was saved
	saved, err := mockRepo.GetProfileByUserID(ctx, "user-123")
	if err != nil {
		t.Fatalf("Profile not found after create: %v", err)
	}
	if saved.Moniker != "alice" {
		t.Errorf("Expected moniker 'alice', got '%s'", saved.Moniker)
	}
}

// Test CreateProfile - duplicate UserID
func TestCreateProfile_DuplicateUserID(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p2 := domain.NewUserProfile("user-123", "bob@example.com", domain.UserTypeIndividual)

	// Create first profile
	err := svc.CreateProfile(ctx, p1)
	if err != nil {
		t.Fatalf("First CreateProfile failed: %v", err)
	}

	// Try to create duplicate
	err = svc.CreateProfile(ctx, p2)
	if err == nil {
		t.Error("Should reject duplicate UserID")
	}
}

// Test CreateProfile - duplicate email
func TestCreateProfile_DuplicateEmail(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p2 := domain.NewUserProfile("user-456", "alice@example.com", domain.UserTypeIndividual)

	// Create first profile
	err := svc.CreateProfile(ctx, p1)
	if err != nil {
		t.Fatalf("First CreateProfile failed: %v", err)
	}

	// Try to create duplicate email
	err = svc.CreateProfile(ctx, p2)
	if err == nil {
		t.Error("Should reject duplicate email")
	}
}

// Test CreateProfile - duplicate platform ID
func TestCreateProfile_DuplicatePlatformID(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p1.PlatformIDs.Slack = "U12345"
	p2 := domain.NewUserProfile("user-456", "bob@example.com", domain.UserTypeIndividual)
	p2.PlatformIDs.Slack = "U12345"

	// Create first profile
	err := svc.CreateProfile(ctx, p1)
	if err != nil {
		t.Fatalf("First CreateProfile failed: %v", err)
	}

	// Try to create duplicate platform ID
	err = svc.CreateProfile(ctx, p2)
	if err == nil {
		t.Error("Should reject duplicate platform ID")
	}
}

// Test CreateProfile - validation failure
func TestCreateProfile_ValidationFailure(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := &domain.UserProfile{
		UserID:       "user-123",
		PrimaryEmail: "invalid-email", // Invalid email format
		UserType:     domain.UserTypeIndividual,
	}

	err := svc.CreateProfile(ctx, p)
	if err == nil {
		t.Error("Should reject invalid profile")
	}
}

// Test GetProfile - success
func TestGetProfile_Success(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	retrieved, err := svc.GetProfile(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if retrieved.UserID != "user-123" {
		t.Errorf("Expected UserID 'user-123', got '%s'", retrieved.UserID)
	}
}

// Test GetProfile - not found
func TestGetProfile_NotFound(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	_, err := svc.GetProfile(ctx, "nonexistent")
	if err == nil {
		t.Error("Should return error for nonexistent profile")
	}
}

// Test UpdateProfile - success
func TestUpdateProfile_Success(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	// Update profile
	updates := map[string]interface{}{
		"moniker":   "alice-updated",
		"firstName": "Alicia",
	}
	err := svc.UpdateProfile(ctx, "user-123", updates)
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}

	// Verify updates
	updated, _ := svc.GetProfile(ctx, "user-123")
	if updated.Moniker != "alice-updated" {
		t.Errorf("Expected moniker 'alice-updated', got '%s'", updated.Moniker)
	}
	if updated.FirstName != "Alicia" {
		t.Errorf("Expected firstName 'Alicia', got '%s'", updated.FirstName)
	}
}

// Test UpdateProfile - invalid field
func TestUpdateProfile_InvalidField(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	// Try to update with invalid email
	updates := map[string]interface{}{
		"primaryEmail": "invalid-email",
	}
	err := svc.UpdateProfile(ctx, "user-123", updates)
	if err == nil {
		t.Error("Should reject invalid email update")
	}
}

// Test DeleteProfile - success
func TestDeleteProfile_Success(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	err := svc.DeleteProfile(ctx, "user-123")
	if err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Verify deletion
	_, err = svc.GetProfile(ctx, "user-123")
	if err == nil {
		t.Error("Profile should be deleted")
	}
}

// Test ListProfiles - success
func TestListProfiles_Success(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create multiple profiles
	for i := 1; i <= 5; i++ {
		p := domain.NewUserProfile("user-"+string(rune('0'+i)), "user"+string(rune('0'+i))+"@example.com", domain.UserTypeIndividual)
		svc.CreateProfile(ctx, p)
	}

	profiles, err := svc.ListProfiles(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if len(profiles) != 5 {
		t.Errorf("Expected 5 profiles, got %d", len(profiles))
	}
}

// Test ListProfiles - pagination
func TestListProfiles_Pagination(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create multiple profiles
	for i := 1; i <= 5; i++ {
		p := domain.NewUserProfile("user-"+string(rune('0'+i)), "user"+string(rune('0'+i))+"@example.com", domain.UserTypeIndividual)
		svc.CreateProfile(ctx, p)
	}

	// Get page 1 (offset 0, limit 2)
	page1, err := svc.ListProfiles(ctx, 0, 2)
	if err != nil {
		t.Fatalf("ListProfiles page1 failed: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("Expected 2 profiles in page1, got %d", len(page1))
	}

	// Get page 2 (offset 2, limit 2)
	page2, err := svc.ListProfiles(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListProfiles page2 failed: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("Expected 2 profiles in page2, got %d", len(page2))
	}
}

// Test audit logging
func TestAuditLogging(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	auditEvents := make([]*domain.AuditEvent, 0)
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents = append(auditEvents, event)
			return nil
		},
	}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create profile (should audit)
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	// Update profile (should audit)
	updates := map[string]interface{}{"moniker": "alice-updated"}
	svc.UpdateProfile(ctx, "user-123", updates)

	// Delete profile (should audit)
	svc.DeleteProfile(ctx, "user-123")

	// Verify audit events
	if len(auditEvents) < 3 {
		t.Errorf("Expected at least 3 audit events, got %d", len(auditEvents))
	}

	// Check audit event actions
	actions := []string{}
	for _, event := range auditEvents {
		actions = append(actions, event.Action)
	}
	t.Logf("Audit actions: %v", actions)
}

// Test GetProfileByEmail
func TestGetProfileByEmail(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	retrieved, err := svc.GetProfileByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetProfileByEmail failed: %v", err)
	}
	if retrieved.UserID != "user-123" {
		t.Errorf("Expected UserID 'user-123', got '%s'", retrieved.UserID)
	}
}

// Test GetProfileByPlatformID
func TestGetProfileByPlatformID(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p.PlatformIDs.Slack = "U12345"
	svc.CreateProfile(ctx, p)

	retrieved, err := svc.GetProfileByPlatformID(ctx, domain.PlatformSlack, "U12345")
	if err != nil {
		t.Fatalf("GetProfileByPlatformID failed: %v", err)
	}
	if retrieved.UserID != "user-123" {
		t.Errorf("Expected UserID 'user-123', got '%s'", retrieved.UserID)
	}
}

// Test CreateProfile - repository save error
func TestCreateProfile_RepositoryError(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockRepo.SaveProfileFunc = func(ctx context.Context, p *domain.UserProfile) error {
		return errors.New("database error")
	}
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)

	err := svc.CreateProfile(ctx, p)
	if err == nil {
		t.Error("Should propagate repository error")
	}
}

// Test UpdateProfile - not found
func TestUpdateProfile_NotFound(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	updates := map[string]interface{}{"moniker": "alice"}

	err := svc.UpdateProfile(ctx, "nonexistent", updates)
	if err == nil {
		t.Error("Should return error for nonexistent profile")
	}
}

// Test UpdateProfile - repository save error
func TestUpdateProfile_RepositoryError(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	// Set up repository to fail on save
	mockRepo.SaveProfileFunc = func(ctx context.Context, p *domain.UserProfile) error {
		return errors.New("database error")
	}

	updates := map[string]interface{}{"moniker": "alice-updated"}
	err := svc.UpdateProfile(ctx, "user-123", updates)
	if err == nil {
		t.Error("Should propagate repository error")
	}
}

// Test UpdateProfile - duplicate email check
func TestUpdateProfile_DuplicateEmail(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p2 := domain.NewUserProfile("user-456", "bob@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p1)
	svc.CreateProfile(ctx, p2)

	// Try to update user-456 to use alice's email
	updates := map[string]interface{}{"primaryEmail": "alice@example.com"}
	err := svc.UpdateProfile(ctx, "user-456", updates)
	if err == nil {
		t.Error("Should reject duplicate email during update")
	}
}

// Test DeleteProfile - not found
func TestDeleteProfile_NotFound(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	err := svc.DeleteProfile(ctx, "nonexistent")
	if err == nil {
		t.Error("Should return error for nonexistent profile")
	}
}

// Test DeleteProfile - repository error
func TestDeleteProfile_RepositoryError(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	svc.CreateProfile(ctx, p)

	// Set up repository to fail on delete
	mockRepo.DeleteProfileFunc = func(ctx context.Context, userID string) error {
		return errors.New("database error")
	}

	err := svc.DeleteProfile(ctx, "user-123")
	if err == nil {
		t.Error("Should propagate repository error")
	}
}

// Test CreateProfile - duplicate Telegram platform ID
func TestCreateProfile_DuplicateTelegramID(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p1.PlatformIDs.Telegram = "123456789"
	p2 := domain.NewUserProfile("user-456", "bob@example.com", domain.UserTypeIndividual)
	p2.PlatformIDs.Telegram = "123456789"

	svc.CreateProfile(ctx, p1)
	err := svc.CreateProfile(ctx, p2)
	if err == nil {
		t.Error("Should reject duplicate Telegram ID")
	}
}

// Test CreateProfile - duplicate CLI platform ID
func TestCreateProfile_DuplicateCLIID(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p1.PlatformIDs.CLI = "alice"
	p2 := domain.NewUserProfile("user-456", "bob@example.com", domain.UserTypeIndividual)
	p2.PlatformIDs.CLI = "alice"

	svc.CreateProfile(ctx, p1)
	err := svc.CreateProfile(ctx, p2)
	if err == nil {
		t.Error("Should reject duplicate CLI ID")
	}
}

// Test ListProfiles - empty result
func TestListProfiles_Empty(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	profiles, err := svc.ListProfiles(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("Expected 0 profiles, got %d", len(profiles))
	}
}

// Test ListProfiles - repository error
func TestListProfiles_RepositoryError(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockRepo.ListProfilesFunc = func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
		return nil, errors.New("database error")
	}
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	_, err := svc.ListProfiles(ctx, 0, 10)
	if err == nil {
		t.Error("Should propagate repository error")
	}
}

// Test GetProfileByEmail - not found
func TestGetProfileByEmail_NotFound(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	_, err := svc.GetProfileByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Error("Should return error for nonexistent email")
	}
}

// Test GetProfileByPlatformID - not found
func TestGetProfileByPlatformID_NotFound(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	_, err := svc.GetProfileByPlatformID(ctx, domain.PlatformSlack, "U99999")
	if err == nil {
		t.Error("Should return error for nonexistent platform ID")
	}
}

// Test UpdateProfile - update platform IDs with duplicate check
func TestUpdateProfile_DuplicatePlatformIDInUpdate(t *testing.T) {
	mockRepo := NewMockUserProfileRepository()
	mockSecurity := &MockSecurityService{}
	svc := profile.NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p1.PlatformIDs.Slack = "U12345"
	p2 := domain.NewUserProfile("user-456", "bob@example.com", domain.UserTypeIndividual)

	svc.CreateProfile(ctx, p1)
	svc.CreateProfile(ctx, p2)

	// Try to update user-456 to use alice's slack ID
	newPlatformIDs := domain.PlatformIdentifiers{
		Slack: "U12345",
	}
	updates := map[string]interface{}{"platformIDs": newPlatformIDs}
	err := svc.UpdateProfile(ctx, "user-456", updates)
	if err == nil {
		t.Error("Should reject duplicate platform ID during update")
	}
}
