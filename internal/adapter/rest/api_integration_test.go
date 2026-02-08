//go:build integration
// +build integration

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nuimanbot/internal/adapter/rest/middleware"
	"nuimanbot/internal/domain"
)

// IntegrationTestServer represents a complete REST API server for testing
type IntegrationTestServer struct {
	router         *mux.Router
	profileService *IntegrationMockProfileService
	botService     *MockBotService
	configService  *MockConfigService
	serverService  *MockServerService
	authMiddleware *middleware.AuthMiddleware
}

// IntegrationMockProfileService extends MockProfileService with GetByAPIKey for auth
type IntegrationMockProfileService struct {
	MockProfileService
	GetByAPIKeyFunc func(ctx context.Context, apiKey string) (*domain.UserProfile, error)
}

func (m *IntegrationMockProfileService) GetByAPIKey(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
	if m.GetByAPIKeyFunc != nil {
		return m.GetByAPIKeyFunc(ctx, apiKey)
	}
	return nil, domain.ErrUserNotFound
}

// NewIntegrationTestServer creates a new test server with all handlers and middleware
func NewIntegrationTestServer() *IntegrationTestServer {
	// Create mock services
	profileService := &IntegrationMockProfileService{}
	botService := &MockBotService{}
	configService := &MockConfigService{}
	serverService := &MockServerService{}

	// Create handlers
	profileHandler := NewProfileHandler(profileService)
	botHandler := NewBotHandler(botService)
	configHandler := NewConfigHandler(configService)
	serverHandler := NewServerHandler(serverService)

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(profileService)

	// Create router
	router := mux.NewRouter()

	// Apply middleware to all admin routes
	router.Use(authMiddleware.Authenticate)
	router.Use(middleware.RequireAdmin)

	// Register routes (handlers already include /api/v1/admin prefix)
	profileHandler.RegisterRoutes(router)
	botHandler.RegisterRoutes(router)
	configHandler.RegisterRoutes(router)
	serverHandler.RegisterRoutes(router)

	return &IntegrationTestServer{
		router:         router,
		profileService: profileService,
		botService:     botService,
		configService:  configService,
		serverService:  serverService,
		authMiddleware: authMiddleware,
	}
}

// TestE2E_UserLifecycle tests complete user lifecycle: Create → Get → Update → Delete
func TestE2E_UserLifecycle(t *testing.T) {
	server := NewIntegrationTestServer()

	// Admin user for authentication
	adminUser := &domain.UserProfile{
		UserID:       "admin-1",
		PrimaryEmail: "admin@example.com",
		Role:         domain.RoleAdmin,
		Enabled:      true,
	}
	adminToken := "admin-api-key-123"

	// Setup auth
	server.profileService.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
		if apiKey == adminToken {
			return adminUser, nil
		}
		return nil, domain.ErrUserNotFound
	}

	// Test data
	testUser := &domain.UserProfile{
		UserID:       "test-user-1",
		PrimaryEmail: "test@example.com",
		FirstName:    "Test",
		LastName:     "User",
		Role:         domain.RoleUser,
		Enabled:      true,
	}

	// Step 1: Create user
	t.Run("Create User", func(t *testing.T) {
		server.profileService.CreateProfileFunc = func(ctx context.Context, profile *domain.UserProfile) error {
			assert.Equal(t, testUser.UserID, profile.UserID)
			assert.Equal(t, testUser.PrimaryEmail, profile.PrimaryEmail)
			return nil
		}

		createReq := CreateProfileRequest{
			UserID:       testUser.UserID,
			PrimaryEmail: testUser.PrimaryEmail,
			FirstName:    testUser.FirstName,
			LastName:     testUser.LastName,
			Role:         testUser.Role,
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest("POST", "/api/v1/admin/profiles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response domain.UserProfile
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, testUser.UserID, response.UserID)
	})

	// Step 2: Get user
	t.Run("Get User", func(t *testing.T) {
		server.profileService.GetProfileFunc = func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			if userID == testUser.UserID {
				return testUser, nil
			}
			return nil, domain.ErrUserNotFound
		}

		req := httptest.NewRequest("GET", "/api/v1/admin/profiles/"+testUser.UserID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response domain.UserProfile
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, testUser.UserID, response.UserID)
	})

	// Step 3: Update user
	t.Run("Update User", func(t *testing.T) {
		newTimezone := "America/New_York"

		server.profileService.UpdateProfileFunc = func(ctx context.Context, userID string, updates map[string]interface{}) error {
			assert.Equal(t, testUser.UserID, userID)
			assert.Equal(t, newTimezone, updates["timezone"])
			return nil
		}

		server.profileService.GetProfileFunc = func(ctx context.Context, userID string) (*domain.UserProfile, error) {
			updated := *testUser
			updated.Timezone = newTimezone
			return &updated, nil
		}

		updateReq := UpdateProfileRequest{
			Timezone: &newTimezone,
		}

		body, _ := json.Marshal(updateReq)
		req := httptest.NewRequest("PUT", "/api/v1/admin/profiles/"+testUser.UserID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response UpdateProfileResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Contains(t, response.UpdatedFields, "timezone")
		assert.Equal(t, newTimezone, response.Profile.Timezone)
	})

	// Step 4: Delete user
	t.Run("Delete User", func(t *testing.T) {
		server.profileService.DeleteProfileFunc = func(ctx context.Context, userID string) error {
			assert.Equal(t, testUser.UserID, userID)
			return nil
		}

		req := httptest.NewRequest("DELETE", "/api/v1/admin/profiles/"+testUser.UserID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// TestE2E_BotLifecycle tests complete bot lifecycle: Create → Enable/Disable → Delete
func TestE2E_BotLifecycle(t *testing.T) {
	server := NewIntegrationTestServer()

	// Admin user for authentication
	adminUser := &domain.UserProfile{
		UserID:       "admin-1",
		PrimaryEmail: "admin@example.com",
		Role:         domain.RoleAdmin,
		Enabled:      true,
	}
	adminToken := "admin-api-key-123"

	server.profileService.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
		if apiKey == adminToken {
			return adminUser, nil
		}
		return nil, domain.ErrUserNotFound
	}

	// Test bot
	testBot := &domain.SlackBotConfig{
		BotID:              "slack-bot-1",
		BotName:            "Test Bot",
		BotType:            domain.BotTypePrivate,
		Enabled:            true,
		SlackBotToken:      "xoxb-test-token",
		SlackAppToken:      "xapp-test-token",
		SlackSigningSecret: "secret-123",
	}

	// Step 1: Create Slack bot
	t.Run("Create Slack Bot", func(t *testing.T) {
		server.botService.CreateSlackBotFunc = func(ctx context.Context, bot *domain.SlackBotConfig) error {
			assert.Equal(t, testBot.BotName, bot.BotName)
			assert.Equal(t, testBot.SlackBotToken, bot.SlackBotToken)
			return nil
		}

		body, _ := json.Marshal(testBot)
		req := httptest.NewRequest("POST", "/api/v1/admin/bots/slack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response domain.SlackBotConfig
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		// Token should be masked
		assert.Contains(t, response.SlackBotToken, "...")
	})

	// Step 2: Disable bot
	t.Run("Disable Slack Bot", func(t *testing.T) {
		server.botService.DisableSlackBotFunc = func(ctx context.Context, botID string) error {
			assert.Equal(t, testBot.BotID, botID)
			return nil
		}

		req := httptest.NewRequest("POST", "/api/v1/admin/bots/slack/"+testBot.BotID+"/disable", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]string
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "disabled", response["status"])
	})

	// Step 3: Enable bot
	t.Run("Enable Slack Bot", func(t *testing.T) {
		server.botService.EnableSlackBotFunc = func(ctx context.Context, botID string) error {
			assert.Equal(t, testBot.BotID, botID)
			return nil
		}

		req := httptest.NewRequest("POST", "/api/v1/admin/bots/slack/"+testBot.BotID+"/enable", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]string
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "enabled", response["status"])
	})

	// Step 4: Delete bot
	t.Run("Delete Slack Bot", func(t *testing.T) {
		server.botService.DeleteSlackBotFunc = func(ctx context.Context, botID string) error {
			assert.Equal(t, testBot.BotID, botID)
			return nil
		}

		req := httptest.NewRequest("DELETE", "/api/v1/admin/bots/slack/"+testBot.BotID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// TestE2E_ConfigManagement tests config update → reload → verify workflow
func TestE2E_ConfigManagement(t *testing.T) {
	server := NewIntegrationTestServer()

	// Admin user for authentication
	adminUser := &domain.UserProfile{
		UserID:       "admin-1",
		PrimaryEmail: "admin@example.com",
		Role:         domain.RoleAdmin,
		Enabled:      true,
	}
	adminToken := "admin-api-key-123"

	server.profileService.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
		if apiKey == adminToken {
			return adminUser, nil
		}
		return nil, domain.ErrUserNotFound
	}

	// Step 1: Get current config
	t.Run("Get Config", func(t *testing.T) {
		server.configService.GetConfigFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return map[string]interface{}{
				"server": map[string]interface{}{
					"port": 8080,
				},
			}, nil
		}

		req := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var config map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&config)
		require.NoError(t, err)
		assert.NotNil(t, config["server"])
	})

	// Step 2: Validate config
	t.Run("Validate Config", func(t *testing.T) {
		server.configService.ValidateConfigFunc = func(ctx context.Context, configData map[string]interface{}) error {
			assert.NotNil(t, configData["server"])
			return nil
		}

		configData := map[string]interface{}{
			"server": map[string]interface{}{
				"port": 9090,
			},
		}

		body, _ := json.Marshal(configData)
		req := httptest.NewRequest("POST", "/api/v1/admin/config/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, true, response["valid"])
	})

	// Step 3: Update config
	t.Run("Update Config", func(t *testing.T) {
		server.configService.UpdateConfigFunc = func(ctx context.Context, updates map[string]interface{}) error {
			assert.NotNil(t, updates["server"])
			return nil
		}

		updates := map[string]interface{}{
			"server": map[string]interface{}{
				"port": 9090,
			},
		}

		body, _ := json.Marshal(updates)
		req := httptest.NewRequest("PUT", "/api/v1/admin/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]string
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "success", response["status"])
	})

	// Step 4: Reload config
	t.Run("Reload Config", func(t *testing.T) {
		server.configService.ReloadConfigFunc = func(ctx context.Context) error {
			return nil
		}

		req := httptest.NewRequest("POST", "/api/v1/admin/config/reload", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]string
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "success", response["status"])
	})
}

// TestE2E_AuthenticationAndAuthorization tests auth flows
func TestE2E_AuthenticationAndAuthorization(t *testing.T) {
	server := NewIntegrationTestServer()

	adminUser := &domain.UserProfile{
		UserID:       "admin-1",
		PrimaryEmail: "admin@example.com",
		Role:         domain.RoleAdmin,
		Enabled:      true,
	}

	regularUser := &domain.UserProfile{
		UserID:       "user-1",
		PrimaryEmail: "user@example.com",
		Role:         domain.RoleUser,
		Enabled:      true,
	}

	disabledUser := &domain.UserProfile{
		UserID:       "disabled-1",
		PrimaryEmail: "disabled@example.com",
		Role:         domain.RoleUser,
		Enabled:      false,
	}

	server.profileService.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
		switch apiKey {
		case "admin-token":
			return adminUser, nil
		case "user-token":
			return regularUser, nil
		case "disabled-token":
			return disabledUser, nil
		default:
			return nil, domain.ErrUserNotFound
		}
	}

	server.profileService.ListProfilesFunc = func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
		return []*domain.UserProfile{adminUser, regularUser}, nil
	}

	// Test 1: No token - Unauthorized
	t.Run("No Token Returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test 2: Invalid token - Forbidden
	t.Run("Invalid Token Returns 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// Test 3: Malformed auth header
	t.Run("Malformed Auth Header Returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
		req.Header.Set("Authorization", "BadFormat token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test 4: Disabled user - Forbidden
	t.Run("Disabled User Returns 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
		req.Header.Set("Authorization", "Bearer disabled-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// Test 5: Regular user trying admin endpoint - Forbidden
	t.Run("Regular User Accessing Admin Endpoint Returns 403", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
		req.Header.Set("Authorization", "Bearer user-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	// Test 6: Admin user accessing admin endpoint - Success
	t.Run("Admin User Accessing Admin Endpoint Succeeds", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/profiles", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestE2E_ServerStatusAndMetrics tests server monitoring endpoints
func TestE2E_ServerStatusAndMetrics(t *testing.T) {
	server := NewIntegrationTestServer()

	adminUser := &domain.UserProfile{
		UserID:       "admin-1",
		PrimaryEmail: "admin@example.com",
		Role:         domain.RoleAdmin,
		Enabled:      true,
	}

	server.profileService.GetByAPIKeyFunc = func(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
		if apiKey == "admin-token" {
			return adminUser, nil
		}
		return nil, domain.ErrUserNotFound
	}

	// Test server status
	t.Run("Get Server Status", func(t *testing.T) {
		server.serverService.GetStatusFunc = func(ctx context.Context) (*ServerStatus, error) {
			return &ServerStatus{
				Uptime:        24 * time.Hour,
				Version:       "1.0.0",
				MemoryUsageMB: 256.5,
				GoVersion:     "1.21",
			}, nil
		}

		req := httptest.NewRequest("GET", "/api/v1/admin/status", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var status ServerStatus
		err := json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", status.Version)
	})

	// Test server metrics
	t.Run("Get Server Metrics", func(t *testing.T) {
		server.serverService.GetMetricsFunc = func(ctx context.Context) (*ServerMetrics, error) {
			return &ServerMetrics{
				RequestsLast24h: 1000,
				ErrorRate:       0.01,
				AvgResponseTime: 150.5,
				ActiveUsers:     25,
				ActiveBots:      5,
			}, nil
		}

		req := httptest.NewRequest("GET", "/api/v1/admin/metrics", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var metrics ServerMetrics
		err := json.NewDecoder(rec.Body).Decode(&metrics)
		require.NoError(t, err)
		assert.Equal(t, 1000, metrics.RequestsLast24h)
		assert.Equal(t, 25, metrics.ActiveUsers)
	})

	// Test server logs
	t.Run("Get Server Logs", func(t *testing.T) {
		server.serverService.GetLogsFunc = func(ctx context.Context, level string, limit int) ([]LogEntry, error) {
			return []LogEntry{
				{
					Timestamp: time.Now(),
					Level:     "info",
					Message:   "Server started",
				},
				{
					Timestamp: time.Now(),
					Level:     "error",
					Message:   "Connection failed",
					UserID:    "user-123",
				},
			}, nil
		}

		req := httptest.NewRequest("GET", "/api/v1/admin/logs?level=all&limit=100", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()

		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "all", response["level"])
		logs := response["logs"].([]interface{})
		assert.Equal(t, 2, len(logs))
	})
}

// Mock services for integration testing

type MockBotService struct {
	CreateSlackBotFunc          func(ctx context.Context, bot *domain.SlackBotConfig) error
	GetSlackBotFunc             func(ctx context.Context, botID string) (*domain.SlackBotConfig, error)
	ListSlackBotsFunc           func(ctx context.Context) ([]*domain.SlackBotConfig, error)
	ListSlackBotsByOwnerFunc    func(ctx context.Context, ownerUserID string) ([]*domain.SlackBotConfig, error)
	UpdateSlackBotFunc          func(ctx context.Context, bot *domain.SlackBotConfig) error
	DeleteSlackBotFunc          func(ctx context.Context, botID string) error
	EnableSlackBotFunc          func(ctx context.Context, botID string) error
	DisableSlackBotFunc         func(ctx context.Context, botID string) error
	CreateTelegramBotFunc       func(ctx context.Context, bot *domain.TelegramBotConfig) error
	GetTelegramBotFunc          func(ctx context.Context, botID string) (*domain.TelegramBotConfig, error)
	ListTelegramBotsFunc        func(ctx context.Context) ([]*domain.TelegramBotConfig, error)
	ListTelegramBotsByOwnerFunc func(ctx context.Context, ownerUserID string) ([]*domain.TelegramBotConfig, error)
	UpdateTelegramBotFunc       func(ctx context.Context, bot *domain.TelegramBotConfig) error
	DeleteTelegramBotFunc       func(ctx context.Context, botID string) error
	EnableTelegramBotFunc       func(ctx context.Context, botID string) error
	DisableTelegramBotFunc      func(ctx context.Context, botID string) error
}

func (m *MockBotService) CreateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	if m.CreateSlackBotFunc != nil {
		return m.CreateSlackBotFunc(ctx, bot)
	}
	return nil
}

func (m *MockBotService) GetSlackBot(ctx context.Context, botID string) (*domain.SlackBotConfig, error) {
	if m.GetSlackBotFunc != nil {
		return m.GetSlackBotFunc(ctx, botID)
	}
	return nil, domain.ErrBotNotFound
}

func (m *MockBotService) ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error) {
	if m.ListSlackBotsFunc != nil {
		return m.ListSlackBotsFunc(ctx)
	}
	return []*domain.SlackBotConfig{}, nil
}

func (m *MockBotService) ListSlackBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.SlackBotConfig, error) {
	if m.ListSlackBotsByOwnerFunc != nil {
		return m.ListSlackBotsByOwnerFunc(ctx, ownerUserID)
	}
	return []*domain.SlackBotConfig{}, nil
}

func (m *MockBotService) UpdateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	if m.UpdateSlackBotFunc != nil {
		return m.UpdateSlackBotFunc(ctx, bot)
	}
	return nil
}

func (m *MockBotService) DeleteSlackBot(ctx context.Context, botID string) error {
	if m.DeleteSlackBotFunc != nil {
		return m.DeleteSlackBotFunc(ctx, botID)
	}
	return nil
}

func (m *MockBotService) EnableSlackBot(ctx context.Context, botID string) error {
	if m.EnableSlackBotFunc != nil {
		return m.EnableSlackBotFunc(ctx, botID)
	}
	return nil
}

func (m *MockBotService) DisableSlackBot(ctx context.Context, botID string) error {
	if m.DisableSlackBotFunc != nil {
		return m.DisableSlackBotFunc(ctx, botID)
	}
	return nil
}

func (m *MockBotService) CreateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	if m.CreateTelegramBotFunc != nil {
		return m.CreateTelegramBotFunc(ctx, bot)
	}
	return nil
}

func (m *MockBotService) GetTelegramBot(ctx context.Context, botID string) (*domain.TelegramBotConfig, error) {
	if m.GetTelegramBotFunc != nil {
		return m.GetTelegramBotFunc(ctx, botID)
	}
	return nil, domain.ErrBotNotFound
}

func (m *MockBotService) ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error) {
	if m.ListTelegramBotsFunc != nil {
		return m.ListTelegramBotsFunc(ctx)
	}
	return []*domain.TelegramBotConfig{}, nil
}

func (m *MockBotService) ListTelegramBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.TelegramBotConfig, error) {
	if m.ListTelegramBotsByOwnerFunc != nil {
		return m.ListTelegramBotsByOwnerFunc(ctx, ownerUserID)
	}
	return []*domain.TelegramBotConfig{}, nil
}

func (m *MockBotService) UpdateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	if m.UpdateTelegramBotFunc != nil {
		return m.UpdateTelegramBotFunc(ctx, bot)
	}
	return nil
}

func (m *MockBotService) DeleteTelegramBot(ctx context.Context, botID string) error {
	if m.DeleteTelegramBotFunc != nil {
		return m.DeleteTelegramBotFunc(ctx, botID)
	}
	return nil
}

func (m *MockBotService) EnableTelegramBot(ctx context.Context, botID string) error {
	if m.EnableTelegramBotFunc != nil {
		return m.EnableTelegramBotFunc(ctx, botID)
	}
	return nil
}

func (m *MockBotService) DisableTelegramBot(ctx context.Context, botID string) error {
	if m.DisableTelegramBotFunc != nil {
		return m.DisableTelegramBotFunc(ctx, botID)
	}
	return nil
}

type MockConfigService struct {
	GetConfigFunc      func(ctx context.Context) (map[string]interface{}, error)
	UpdateConfigFunc   func(ctx context.Context, updates map[string]interface{}) error
	ReloadConfigFunc   func(ctx context.Context) error
	ValidateConfigFunc func(ctx context.Context, configData map[string]interface{}) error
}

func (m *MockConfigService) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(ctx)
	}
	return map[string]interface{}{}, nil
}

func (m *MockConfigService) UpdateConfig(ctx context.Context, updates map[string]interface{}) error {
	if m.UpdateConfigFunc != nil {
		return m.UpdateConfigFunc(ctx, updates)
	}
	return nil
}

func (m *MockConfigService) ReloadConfig(ctx context.Context) error {
	if m.ReloadConfigFunc != nil {
		return m.ReloadConfigFunc(ctx)
	}
	return nil
}

func (m *MockConfigService) ValidateConfig(ctx context.Context, configData map[string]interface{}) error {
	if m.ValidateConfigFunc != nil {
		return m.ValidateConfigFunc(ctx, configData)
	}
	return nil
}

type MockServerService struct {
	GetStatusFunc  func(ctx context.Context) (*ServerStatus, error)
	GetMetricsFunc func(ctx context.Context) (*ServerMetrics, error)
	GetLogsFunc    func(ctx context.Context, level string, limit int) ([]LogEntry, error)
}

func (m *MockServerService) GetStatus(ctx context.Context) (*ServerStatus, error) {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc(ctx)
	}
	return &ServerStatus{}, nil
}

func (m *MockServerService) GetMetrics(ctx context.Context) (*ServerMetrics, error) {
	if m.GetMetricsFunc != nil {
		return m.GetMetricsFunc(ctx)
	}
	return &ServerMetrics{}, nil
}

func (m *MockServerService) GetLogs(ctx context.Context, level string, limit int) ([]LogEntry, error) {
	if m.GetLogsFunc != nil {
		return m.GetLogsFunc(ctx, level, limit)
	}
	return []LogEntry{}, nil
}
