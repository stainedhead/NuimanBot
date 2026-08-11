package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/chat"
	"nuimanbot/internal/usecase/chats"
)

// errChatNotFound is returned to the terminal user when a requested Chat
// doesn't exist or isn't owned by ownerUserID (domain.ErrNotFound from
// chats.Service) — a distinct, stable message rather than the
// wrap-and-repeat "chat not found: not found" a naive %w wrap would produce.
var errChatNotFound = errors.New("chat not found")

// ChatCommandHandler handles the Chats environment's CLI commands
// (FR-011-016): list, show, new, send, delete, export.
type ChatCommandHandler struct {
	service *chats.Service
}

// NewChatCommandHandler creates a new Chats environment command handler
// backed by service.
func NewChatCommandHandler(service *chats.Service) *ChatCommandHandler {
	return &ChatCommandHandler{service: service}
}

// Handle implements EnvCommandHandler.
func (h *ChatCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	return h.HandleChatCommand(ctx, currentUser, ownerUserID, input)
}

// HandleChatCommand parses and executes a "/chat ..." command line, using
// ownerUserID (AD-5: the authenticated session's Username) to scope every
// call into the Chats service. currentUser is accepted to satisfy
// EnvCommandHandler's shape but Chats has no admin-only subcommands (any
// authenticated user may use it), so it is otherwise unused here.
func (h *ChatCommandHandler) HandleChatCommand(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return h.showHelp(), nil
	}

	subcommand := fields[1]
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0])), subcommand))

	switch subcommand {
	case "list":
		return h.listChats(ctx, ownerUserID)
	case "show":
		return h.showChat(ctx, ownerUserID, rest)
	case "new":
		return h.newChat(ctx, ownerUserID, rest)
	case "send":
		return h.sendMessage(ctx, ownerUserID, rest)
	case "delete":
		return h.deleteChat(ctx, ownerUserID, rest)
	case "export":
		return h.exportChat(ctx, ownerUserID, rest)
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown chat command: %s\nUse '/chat help' for usage information.", subcommand), nil
	}
}

// listChats lists ownerUserID's Chats (FR-011).
func (h *ChatCommandHandler) listChats(ctx context.Context, ownerUserID string) (string, error) {
	summaries, err := h.service.ListChats(ctx, ownerUserID)
	if err != nil {
		return "", fmt.Errorf("failed to list chats: %w", err)
	}
	if len(summaries) == 0 {
		return "No chats found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d chat(s):\n\n", len(summaries))
	for i, c := range summaries {
		fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, c.Name, c.ID)
		fmt.Fprintf(&b, "   Updated: %s | Messages: %d\n", c.UpdatedAt.Format("2006-01-02 15:04:05"), c.MessageCount)
	}
	return b.String(), nil
}

// showChat displays a Chat's message history (FR-012).
// Usage: /chat show <id>
func (h *ChatCommandHandler) showChat(ctx context.Context, ownerUserID, rest string) (string, error) {
	id, _ := splitFirstToken(rest)
	if id == "" {
		return "Usage: /chat show <id>", nil
	}

	conv, err := h.service.GetChat(ctx, ownerUserID, id)
	if err != nil {
		return "", wrapChatServiceError("show chat", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Chat: %s (%s)\n", conv.Name, conv.ID)
	fmt.Fprintf(&b, "Created: %s | Updated: %s\n\n", conv.CreatedAt.Format("2006-01-02 15:04:05"), conv.UpdatedAt.Format("2006-01-02 15:04:05"))
	if len(conv.Messages) == 0 {
		b.WriteString("No messages yet.\n")
	}
	for _, m := range conv.Messages {
		fmt.Fprintf(&b, "[%s] %s: %s\n", m.Timestamp.Format("15:04:05"), m.Role, m.Content)
	}
	return b.String(), nil
}

// newChat creates a new Chat and sends message as its first message
// (FR-013). Usage: /chat new <message>
func (h *ChatCommandHandler) newChat(ctx context.Context, ownerUserID, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "Usage: /chat new <message>", nil
	}

	conv, err := h.service.CreateChat(ctx, ownerUserID, message)
	if err != nil {
		return "", fmt.Errorf("failed to create chat: %w", err)
	}
	return fmt.Sprintf("✓ Chat created: %s (%s)", conv.Name, conv.ID), nil
}

// sendMessage sends a message to an existing Chat (FR-014).
// Usage: /chat send <id> <message>
func (h *ChatCommandHandler) sendMessage(ctx context.Context, ownerUserID, rest string) (string, error) {
	id, message := splitFirstToken(rest)
	if id == "" || message == "" {
		return "Usage: /chat send <id> <message>", nil
	}

	if err := h.service.AppendUserMessage(ctx, ownerUserID, id, message); err != nil {
		return "", wrapChatServiceError("send message", err)
	}
	return fmt.Sprintf("✓ Message sent to chat %s", id), nil
}

// deleteChat immediately deletes a Chat (FR-015).
// Usage: /chat delete <id>
func (h *ChatCommandHandler) deleteChat(ctx context.Context, ownerUserID, rest string) (string, error) {
	id, _ := splitFirstToken(rest)
	if id == "" {
		return "Usage: /chat delete <id>", nil
	}

	if err := h.service.DeleteChat(ctx, ownerUserID, id); err != nil {
		return "", wrapChatServiceError("delete chat", err)
	}
	return fmt.Sprintf("✓ Chat %s deleted", id), nil
}

// exportChat exports a Chat's transcript (FR-016). Usage: /chat export <id>
// [format] [path]. format is "json" or "markdown" (default "markdown"). With
// no path, the transcript text is returned directly as the command's
// response; with a path, it is written to that local file (0600, matching
// this package's existing session-file convention in auth_commands.go) and
// a confirmation is returned instead.
func (h *ChatCommandHandler) exportChat(ctx context.Context, ownerUserID, rest string) (string, error) {
	id, afterID := splitFirstToken(rest)
	if id == "" {
		return "Usage: /chat export <id> [json|markdown] [path]", nil
	}

	format := chat.ExportFormatMarkdown
	var path string
	if fields := strings.Fields(afterID); len(fields) > 0 {
		parsed, ok := parseExportFormat(fields[0])
		if !ok {
			return fmt.Sprintf("Unknown export format: %s (use 'json' or 'markdown')", fields[0]), nil
		}
		format = parsed
		if len(fields) > 1 {
			path = fields[1]
		}
	}

	content, err := h.service.ExportChat(ctx, ownerUserID, id, format)
	if err != nil {
		return "", wrapChatServiceError("export chat", err)
	}

	if path == "" {
		return content, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("failed to write export file: %w", err)
	}
	return fmt.Sprintf("✓ Chat %s exported to %s", id, path), nil
}

// showHelp returns help text for Chats environment commands.
func (h *ChatCommandHandler) showHelp() string {
	return `Chat Commands:

  /chat list
    List your chats

  /chat show <id>
    Show a chat's message history

  /chat new <message>
    Create a new chat and send the first message

  /chat send <id> <message>
    Send a message to an existing chat

  /chat delete <id>
    Delete a chat immediately

  /chat export <id> [json|markdown] [path]
    Export a chat's transcript (default format: markdown)
    Without a path, prints the transcript; with a path, writes it to that local file

  /chat help
    Show this help message
`
}

// parseExportFormat maps a user-supplied format token to a
// chat.ExportFormat, accepting "md" as a shorthand for "markdown". ok is
// false for an unrecognized token.
func parseExportFormat(s string) (format chat.ExportFormat, ok bool) {
	switch strings.ToLower(s) {
	case "json":
		return chat.ExportFormatJSON, true
	case "markdown", "md":
		return chat.ExportFormatMarkdown, true
	default:
		return "", false
	}
}

// wrapChatServiceError translates an error from the Chats service into a
// terminal-facing error: domain.ErrNotFound (used uniformly for both
// "doesn't exist" and "not owned by ownerUserID" — chats.Service never
// discloses which) becomes the stable errChatNotFound; anything else is
// wrapped with action context.
func wrapChatServiceError(action string, err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return errChatNotFound
	}
	return fmt.Errorf("failed to %s: %w", action, err)
}

// splitFirstToken splits s (already whitespace-trimmed content) into its
// first whitespace-delimited token and the raw remainder, preserving any
// internal whitespace within the remainder (unlike strings.Fields, which
// would collapse it) — needed so message bodies (e.g. "/chat send <id>
// <message>") round-trip exactly as typed.
func splitFirstToken(s string) (first, rest string) {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	first = fields[0]
	rest = strings.TrimSpace(strings.TrimPrefix(s, first))
	return first, rest
}
