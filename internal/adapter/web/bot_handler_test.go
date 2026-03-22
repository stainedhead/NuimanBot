package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

// MockBotService is a mock implementation of BotService for testing
type MockBotService struct {
	bots map[string]*BotConfig
}

func NewMockBotService() *MockBotService {
	return &MockBotService{
		bots: make(map[string]*BotConfig),
	}
}

func (m *MockBotService) CreateBot(ctx context.Context, bot *BotConfig) error {
	m.bots[bot.ID] = bot
	return nil
}

func (m *MockBotService) GetBot(ctx context.Context, botID string) (*BotConfig, error) {
	bot, exists := m.bots[botID]
	if !exists {
		return nil, domain.ErrBotNotFound
	}
	return bot, nil
}

func (m *MockBotService) UpdateBot(ctx context.Context, botID string, updates map[string]interface{}) error {
	bot, exists := m.bots[botID]
	if !exists {
		return domain.ErrBotNotFound
	}
	if name, ok := updates["name"].(string); ok {
		bot.Name = name
	}
	return nil
}

func (m *MockBotService) DeleteBot(ctx context.Context, botID string) error {
	if _, exists := m.bots[botID]; !exists {
		return domain.ErrBotNotFound
	}
	delete(m.bots, botID)
	return nil
}

func (m *MockBotService) ListBots(ctx context.Context) ([]*BotConfig, error) {
	bots := make([]*BotConfig, 0, len(m.bots))
	for _, bot := range m.bots {
		bots = append(bots, bot)
	}
	return bots, nil
}

// TestBotsPageRequiresAuth tests that bots page requires authentication
func TestBotsPageRequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/bots", nil)
	w := httptest.NewRecorder()

	server.handleBots(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect to login, got status %d", w.Code)
	}
}

// TestBotsPageWithAuth tests that authenticated users can access bots page
func TestBotsPageWithAuth(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	botService := NewMockBotService()
	server.SetBotService(botService)

	// Create test user and session
	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	// Add a test bot
	if err := botService.CreateBot(context.Background(), &BotConfig{
		ID:       "bot1",
		Name:     "Test Bot",
		Platform: domain.PlatformSlack,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("CreateBot failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/bots", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleBots(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Bots") {
		t.Error("expected bots page to contain 'Bots'")
	}
}

// TestBotDelete tests deleting a bot
func TestBotDelete(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	botService := NewMockBotService()
	server.SetBotService(botService)

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	// Create a test bot
	if err := botService.CreateBot(context.Background(), &BotConfig{
		ID:       "deletebot",
		Name:     "Delete Me",
		Platform: domain.PlatformSlack,
	}); err != nil {
		t.Fatalf("CreateBot failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/deletebot/delete", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleBotDelete(w, req)

	// Should redirect after deletion
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status, got %d", w.Code)
	}

	// Verify bot was deleted
	_, err := botService.GetBot(context.Background(), "deletebot")
	if err != domain.ErrBotNotFound {
		t.Error("expected bot to be deleted")
	}
}
