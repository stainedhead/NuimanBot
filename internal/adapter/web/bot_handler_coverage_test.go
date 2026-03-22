package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

// errorBotService is a BotService that always returns errors.
type errorBotService struct{}

func (e *errorBotService) CreateBot(_ context.Context, _ *BotConfig) error {
	return domain.ErrBotNotFound // reuse a domain error
}
func (e *errorBotService) GetBot(_ context.Context, _ string) (*BotConfig, error) {
	return nil, domain.ErrBotNotFound
}
func (e *errorBotService) UpdateBot(_ context.Context, _ string, _ map[string]interface{}) error {
	return domain.ErrBotNotFound
}
func (e *errorBotService) DeleteBot(_ context.Context, _ string) error {
	return domain.ErrBotNotFound
}
func (e *errorBotService) ListBots(_ context.Context) ([]*BotConfig, error) {
	return nil, domain.ErrBotNotFound
}

// TestHandleBotCreate_RequiresAuth verifies redirect when unauthenticated.
func TestHandleBotCreate_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/create", nil)
	w := httptest.NewRecorder()

	server.handleBotCreate(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleBotCreate_NonAdminForbidden verifies 403 for non-admin users.
func TestHandleBotCreate_NonAdminForbidden(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("regular", "pass", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("regular", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/create", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotCreate(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestHandleBotCreate_GET verifies the create form is shown.
func TestHandleBotCreate_GET(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/create", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotCreate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Create Bot") {
		t.Error("expected body to contain 'Create Bot'")
	}
}

// TestHandleBotCreate_POST_Success verifies bot creation with POST.
func TestHandleBotCreate_POST_Success(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	botService := NewMockBotService()
	server.SetBotService(botService)

	form := url.Values{}
	form.Set("botID", "newbot")
	form.Set("name", "My Bot")

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotCreate(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}

	_, err := botService.GetBot(context.Background(), "newbot")
	if err != nil {
		t.Errorf("expected bot to be created, got error: %v", err)
	}
}

// TestHandleBotCreate_POST_ServiceError verifies 500 on service failure.
func TestHandleBotCreate_POST_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetBotService(&errorBotService{})

	form := url.Values{}
	form.Set("botID", "failbot")
	form.Set("name", "Fail Bot")

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotCreate(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestHandleBotCreate_MethodNotAllowed verifies 405 for PATCH.
func TestHandleBotCreate_MethodNotAllowed(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodPatch, "/admin/bots/create", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotCreate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleBotEdit_RequiresAuth verifies redirect when unauthenticated.
func TestHandleBotEdit_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/bot1/edit", nil)
	w := httptest.NewRecorder()

	server.handleBotEdit(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleBotEdit_InvalidPath verifies 400 for short path.
func TestHandleBotEdit_InvalidPath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/x/y", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotEdit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestHandleBotEdit_GET verifies the edit form is shown.
func TestHandleBotEdit_GET(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/mybot/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotEdit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestHandleBotEdit_POST_Success verifies bot edit POST succeeds.
func TestHandleBotEdit_POST_Success(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	botService := NewMockBotService()
	server.SetBotService(botService)

	if err := botService.CreateBot(context.Background(), &BotConfig{
		ID:      "editbot",
		Name:    "Old Name",
		Enabled: true,
	}); err != nil {
		t.Fatalf("CreateBot: %v", err)
	}

	form := url.Values{}
	form.Set("name", "New Name")

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/editbot/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotEdit(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleBotEdit_POST_ServiceError verifies 500 when UpdateBot fails.
func TestHandleBotEdit_POST_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetBotService(&errorBotService{})

	form := url.Values{}
	form.Set("name", "Bad Update")

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/failbot/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotEdit(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestHandleBotEdit_MethodNotAllowed verifies 405 for DELETE.
func TestHandleBotEdit_MethodNotAllowed(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/bots/mybot/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotEdit(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleBotDelete_RequiresAuth verifies redirect when unauthenticated.
func TestHandleBotDelete_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/bot1/delete", nil)
	w := httptest.NewRecorder()

	server.handleBotDelete(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleBotDelete_NonAdminForbidden verifies 403 for non-admin.
func TestHandleBotDelete_NonAdminForbidden(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("regular", "pass", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("regular", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/bot1/delete", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotDelete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestHandleBotDelete_InvalidPath verifies 400 for short paths.
func TestHandleBotDelete_InvalidPath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodPost, "/x/y", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotDelete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestHandleBotDelete_ServiceError verifies 500 when DeleteBot fails.
func TestHandleBotDelete_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetBotService(&errorBotService{})

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/failbot/delete", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBotDelete(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestHandleBots_ServiceError verifies the page still renders when ListBots fails.
func TestHandleBots_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetBotService(&errorBotService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/bots", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleBots(w, req)

	// Should still render the page (with empty bot list) despite service error.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 despite service error, got %d", w.Code)
	}
}
