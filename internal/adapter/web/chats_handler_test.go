package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/chat"
)

// MockChatsService is a test double for ChatsService.
type MockChatsService struct {
	chats map[string]*domain.Conversation // chatID -> chat

	createErr error
	getErr    error
	sendErr   error
}

func NewMockChatsService() *MockChatsService {
	return &MockChatsService{chats: make(map[string]*domain.Conversation)}
}

func (m *MockChatsService) CreateChat(_ context.Context, ownerUserID, firstMessageText string) (*domain.Conversation, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	now := time.Now()
	conv := &domain.Conversation{
		ID:        "chat-" + ownerUserID + "-" + firstMessageText,
		UserID:    ownerUserID,
		Name:      domain.DeriveConversationName(firstMessageText, now),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if strings.TrimSpace(firstMessageText) != "" {
		conv.Messages = []domain.StoredMessage{{ID: "m1", Role: "user", Content: firstMessageText, Timestamp: now}}
	}
	m.chats[conv.ID] = conv
	return conv, nil
}

func (m *MockChatsService) ListChats(_ context.Context, ownerUserID string) ([]*domain.ConversationSummary, error) {
	out := make([]*domain.ConversationSummary, 0)
	for _, c := range m.chats {
		if c.UserID == ownerUserID {
			out = append(out, &domain.ConversationSummary{ID: c.ID, UserID: c.UserID, Name: c.Name, UpdatedAt: c.UpdatedAt, MessageCount: len(c.Messages)})
		}
	}
	return out, nil
}

func (m *MockChatsService) GetChat(_ context.Context, ownerUserID, chatID string) (*domain.Conversation, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.chats[chatID]
	if !ok || c.UserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (m *MockChatsService) DeleteChat(_ context.Context, ownerUserID, chatID string) error {
	c, ok := m.chats[chatID]
	if !ok || c.UserID != ownerUserID {
		return domain.ErrNotFound
	}
	delete(m.chats, chatID)
	return nil
}

func (m *MockChatsService) AppendUserMessage(_ context.Context, ownerUserID, chatID, content string) error {
	c, ok := m.chats[chatID]
	if !ok || c.UserID != ownerUserID {
		return domain.ErrNotFound
	}
	c.Messages = append(c.Messages, domain.StoredMessage{ID: "m2", Role: "user", Content: content, Timestamp: time.Now()})
	return nil
}

// SendMessage mimics AppendUserMessage plus a canned agent reply — good
// enough for handler-level tests (does handleChatMessage call the right
// method, redirect, and surface errors correctly); chats.Service's own
// SendMessage semantics are covered directly in internal/usecase/chats.
func (m *MockChatsService) SendMessage(_ context.Context, ownerUserID, chatID, content, _ string) (domain.OutgoingMessage, error) {
	if m.sendErr != nil {
		return domain.OutgoingMessage{}, m.sendErr
	}
	c, ok := m.chats[chatID]
	if !ok || c.UserID != ownerUserID {
		return domain.OutgoingMessage{}, domain.ErrNotFound
	}
	c.Messages = append(c.Messages, domain.StoredMessage{ID: "m2", Role: "user", Content: content, Timestamp: time.Now()})
	return domain.OutgoingMessage{Content: "mock agent reply"}, nil
}

func (m *MockChatsService) ExportChat(_ context.Context, ownerUserID, chatID string, format chat.ExportFormat) (string, error) {
	c, ok := m.chats[chatID]
	if !ok || c.UserID != ownerUserID {
		return "", domain.ErrNotFound
	}
	return "exported:" + string(format) + ":" + c.ID, nil
}

// newAuthenticatedChatsRequest builds a request carrying a valid session
// cookie for username with RoleUser, wired to a server with a mock
// ChatsService.
func newChatsTestServer(t *testing.T) (*Server, *MockChatsService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockChatsService()
	server.SetChatsService(mock)
	return server, mock
}

func sessionCookieFor(server *Server, username, role string) *http.Cookie {
	sessionID := server.auth.CreateSession(username, role)
	return &http.Cookie{Name: sessionCookieName, Value: sessionID}
}

func TestHandleChats_RequiresAuth(t *testing.T) {
	server, _ := newChatsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/chats", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleChats_ListEmpty(t *testing.T) {
	server, _ := newChatsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/chats", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleChats_CreateAndRedirect(t *testing.T) {
	server, mock := newChatsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	// CSRF token must be obtained from a rendered page first (GenerateCSRFToken
	// registers the token server-side).
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("first_message", "plan my trip")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chats", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d, body: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/admin/chats/") {
		t.Fatalf("expected redirect to a chat detail page, got %q", loc)
	}
	if len(mock.chats) != 1 {
		t.Fatalf("expected 1 chat created, got %d", len(mock.chats))
	}
}

func TestHandleChats_CreateInvalidCSRFRejected(t *testing.T) {
	server, mock := newChatsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	form := url.Values{}
	form.Set("first_message", "plan my trip")
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/chats", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if len(mock.chats) != 0 {
		t.Fatal("expected no chat to be created with an invalid CSRF token")
	}
}

func TestHandleChatDetail_CrossOwnerReturns404(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "bob", Name: "Bob's secret"}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chats/chat-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access (not 403 — existence must not be disclosed), got %d", w.Code)
	}
}

func TestHandleChatDetail_OwnerSuccess(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "alice", Name: "Trip planning chat"}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chats/chat-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Trip planning chat") {
		t.Fatal("expected chat name to appear in rendered page")
	}
}

func TestHandleChatDelete_CrossOwnerReturns404(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "bob"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chats/chat-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if _, ok := mock.chats["chat-1"]; !ok {
		t.Fatal("expected bob's chat to remain undeleted")
	}
}

func TestHandleChatDelete_OwnerSuccess(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chats/chat-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if _, ok := mock.chats["chat-1"]; ok {
		t.Fatal("expected chat to be deleted")
	}
}

func TestHandleChatMessage_AppendsAndRedirects(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("content", "another message")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chats/chat-1/message", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if len(mock.chats["chat-1"].Messages) != 1 {
		t.Fatalf("expected 1 message appended, got %d", len(mock.chats["chat-1"].Messages))
	}
}

func TestHandleChatExport_JSONAndMarkdown(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "alice"}
	cookie := sessionCookieFor(server, "alice", "user")

	for _, format := range []string{"json", "markdown"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/chats/chat-1/export?format="+format, nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("format %s: expected 200, got %d", format, w.Code)
		}
		if !strings.Contains(w.Body.String(), "exported:"+format) {
			t.Fatalf("format %s: unexpected body: %s", format, w.Body.String())
		}
		if w.Header().Get("Content-Disposition") == "" {
			t.Fatalf("format %s: expected Content-Disposition header for download", format)
		}
	}
}

func TestHandleChatExport_CrossOwnerReturns404(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "bob"}
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/chats/chat-1/export", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleChatSubroutes_UnknownActionNotFound(t *testing.T) {
	server, mock := newChatsTestServer(t)
	mock.chats["chat-1"] = &domain.Conversation{ID: "chat-1", UserID: "alice"}
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/chats/chat-1/bogus", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown subroute action, got %d", w.Code)
	}
}

func TestHandleChatSubroutes_EmptyIDNotFound(t *testing.T) {
	server, _ := newChatsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chats/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty chat ID, got %d", w.Code)
	}
}

func TestChatsService_NotConfigured(t *testing.T) {
	server := NewServer(":0") // no SetChatsService call
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chats", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when ChatsService is unconfigured, got %d", w.Code)
	}
}

func TestChatIDAndActionFromPath(t *testing.T) {
	cases := []struct {
		path       string
		wantID     string
		wantAction string
	}{
		{"/admin/chats/abc", "abc", ""},
		{"/admin/chats/abc/message", "abc", "message"},
		{"/admin/chats/abc/delete", "abc", "delete"},
		{"/admin/chats/", "", ""},
		{"/admin/chats/abc/too/many", "", ""},
	}
	for _, tc := range cases {
		id, action := chatIDAndActionFromPath(tc.path)
		if id != tc.wantID || action != tc.wantAction {
			t.Errorf("path %q: expected (%q, %q), got (%q, %q)", tc.path, tc.wantID, tc.wantAction, id, action)
		}
	}
}

// Ensure MockChatsService satisfies the interface at compile time.
var _ ChatsService = (*MockChatsService)(nil)
