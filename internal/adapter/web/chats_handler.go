package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/chat"
)

// ChatsService is the interface the web admin's Chats environment (FR-011–
// FR-016) depends on. Production wiring composes internal/usecase/chats.Service
// (see cmd/nuimanbot/main.go). ownerUserID throughout is the current
// session's Username (see getCurrentUser) — the stable per-user identifier
// this package uses for owned-resource scoping; session.ID is a per-session
// token, not a stable user identifier, and must never be used for this
// purpose (see implementation-notes.md).
type ChatsService interface {
	CreateChat(ctx context.Context, ownerUserID, firstMessageText string) (*domain.Conversation, error)
	ListChats(ctx context.Context, ownerUserID string) ([]*domain.ConversationSummary, error)
	GetChat(ctx context.Context, ownerUserID, chatID string) (*domain.Conversation, error)
	DeleteChat(ctx context.Context, ownerUserID, chatID string) error
	AppendUserMessage(ctx context.Context, ownerUserID, chatID, content string) error
	ExportChat(ctx context.Context, ownerUserID, chatID string, format chat.ExportFormat) (string, error)
}

// SetChatsService sets the Chats environment's service (optional; routes
// degrade to a 500 with a clear message when unset, matching this package's
// existing ConfirmationService/BotService convention).
func (s *Server) SetChatsService(svc ChatsService) {
	s.chatsService = svc
}

// ChatsPageData is the template data for the Chats list/create page.
type ChatsPageData struct {
	*BaseData
	Chats     []*domain.ConversationSummary
	CSRFToken string
}

// ChatDetailPageData is the template data for a single Chat's detail page.
type ChatDetailPageData struct {
	*BaseData
	Chat      *domain.Conversation
	CSRFToken string
}

// handleChats lists the current user's Chats (GET) and creates a new one (POST).
func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.chatsService == nil {
		http.Error(w, "Chats service not configured", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		s.handleChatCreate(w, r, user)
		return
	}

	chatsList, err := s.chatsService.ListChats(r.Context(), user.Username)
	if err != nil {
		slog.Error("Failed to list chats", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &ChatsPageData{
		BaseData:  s.baseDataFor(user, "Chats", "chats"),
		Chats:     chatsList,
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "chats.html", data); err != nil {
		slog.Error("Failed to render chats template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleChatCreate creates a new Chat from a form-posted first message and
// redirects to its detail page (FR-011/FR-012).
func (s *Server) handleChatCreate(w http.ResponseWriter, r *http.Request, user *User) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	firstMessage := sanitizedFormValue(r, "first_message")
	conv, err := s.chatsService.CreateChat(r.Context(), user.Username, firstMessage)
	if err != nil {
		slog.Error("Failed to create chat", "error", err)
		s.Error500(w, r, err)
		return
	}

	http.Redirect(w, r, "/admin/chats/"+conv.ID, http.StatusFound)
}

// handleChatSubroutes dispatches /admin/chats/{id}[/message|/delete|/export].
func (s *Server) handleChatSubroutes(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.chatsService == nil {
		http.Error(w, "Chats service not configured", http.StatusInternalServerError)
		return
	}

	id, action := chatIDAndActionFromPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		s.handleChatDetail(w, r, user, id)
	case "message":
		s.handleChatMessage(w, r, user, id)
	case "delete":
		s.handleChatDelete(w, r, user, id)
	case "export":
		s.handleChatExport(w, r, user, id)
	default:
		http.NotFound(w, r)
	}
}

// handleChatDetail renders a single Chat's message thread (FR-013).
// A chat that doesn't exist or belongs to a different user renders 404 —
// never a distinct "forbidden" response — per spec.md Edge Case #10.
func (s *Server) handleChatDetail(w http.ResponseWriter, r *http.Request, user *User, id string) {
	conv, err := s.chatsService.GetChat(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get chat", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &ChatDetailPageData{
		BaseData:  s.baseDataFor(user, conv.Name, "chats"),
		Chat:      conv,
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "chat_detail.html", data); err != nil {
		slog.Error("Failed to render chat detail template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleChatMessage appends a user message to a Chat the caller owns.
func (s *Server) handleChatMessage(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	content := sanitizedFormValue(r, "content")
	if content != "" {
		if err := s.chatsService.AppendUserMessage(r.Context(), user.Username, id, content); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			slog.Error("Failed to append chat message", "error", err)
			s.Error500(w, r, err)
			return
		}
	}
	http.Redirect(w, r, "/admin/chats/"+id, http.StatusFound)
}

// handleChatDelete immediately and manually deletes a Chat (FR-015).
func (s *Server) handleChatDelete(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if err := s.chatsService.DeleteChat(r.Context(), user.Username, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to delete chat", "error", err)
		s.Error500(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/chats", http.StatusFound)
}

// handleChatExport streams a Chat's transcript as a download (FR-016).
// format is selected via ?format=json|markdown, defaulting to markdown.
func (s *Server) handleChatExport(w http.ResponseWriter, r *http.Request, user *User, id string) {
	format := chat.ExportFormatMarkdown
	ext := "md"
	contentType := "text/markdown; charset=utf-8"
	if r.URL.Query().Get("format") == "json" {
		format = chat.ExportFormatJSON
		ext = "json"
		contentType = "application/json"
	}

	content, err := s.chatsService.ExportChat(r.Context(), user.Username, id, format)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to export chat", "error", err)
		s.Error500(w, r, err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="chat-`+id+`.`+ext+`"`)
	if _, err := w.Write([]byte(content)); err != nil {
		slog.Error("Failed to write chat export response", "error", err)
	}
}

// validCSRF checks and consumes the request's csrf_token form value.
func (s *Server) validCSRF(r *http.Request) bool {
	return s.auth != nil && s.auth.ValidateCSRFToken(r.FormValue("csrf_token"))
}

// baseDataFor builds a *BaseData populated for an authenticated user,
// shared by every new environment handler (Chats and, by the same
// convention, Projects/Jobs/Chores/History/Memories/Settings).
func (s *Server) baseDataFor(user *User, title, activePage string) *BaseData {
	bd := NewBaseData(title, activePage)
	bd.WithUser(user)
	return bd
}

// chatIDAndActionFromPath parses "/admin/chats/{id}" or
// "/admin/chats/{id}/{action}" into (id, action). action is "" for the
// bare detail path.
func chatIDAndActionFromPath(path string) (id string, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "chats", "{id}"] or ["admin", "chats", "{id}", "{action}"]
	if len(parts) < 3 || parts[2] == "" {
		return "", ""
	}
	if len(parts) == 3 {
		return parts[2], ""
	}
	if len(parts) == 4 {
		return parts[2], parts[3]
	}
	return "", ""
}
