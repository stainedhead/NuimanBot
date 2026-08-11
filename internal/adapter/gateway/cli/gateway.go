package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time" // Added for time.Now()

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/auth"
)

// SkillCommandHandler defines the interface for handling skill commands.
type SkillCommandHandler interface {
	Execute(ctx context.Context, skillName string, args []string) error
	List(ctx context.Context) error
	Describe(ctx context.Context, skillName string) error
}

// PlatformUIDSetter is implemented by skill handlers that need to be told
// the authenticated user's identity once login completes (skill-invoked
// chat messages must be attributed to the same identity as plain-text chat
// messages, AD-5/FR-007). *SkillHandler implements this.
type PlatformUIDSetter interface {
	SetPlatformUID(uid string)
}

// unauthenticatedPlatformUID is used only if a message is somehow processed
// before EnsureAuthenticated completes (should not happen — Start returns
// early on authentication failure). Kept distinct from any real username so
// it's obviously a placeholder if it ever surfaces in logs/data.
const unauthenticatedPlatformUID = "cli_unauthenticated"

// Gateway implements domain.Gateway for the command-line interface.
type Gateway struct {
	Cfg            *config.CLIConfig
	messageHandler domain.MessageHandler
	adminHandler   *AdminCommandHandler
	profileHandler *AdminProfileCommandHandler
	botHandler     *AdminBotCommandHandler
	configHandler  *AdminConfigCommandHandler
	memoryHandler  *MemoryCommandHandler
	skillHandler   SkillCommandHandler
	authHandler    *AuthCommandHandler // login/logout flow (FR-001-FR-007); nil disables auth-gating entirely
	currentUser    *domain.User        // Current CLI user for admin commands
	currentSession *auth.Session       // Current authenticated session, set by EnsureAuthenticated
	// stdin/stdout for testing purposes
	Reader io.Reader
	Writer io.Writer
	cancel context.CancelFunc // For stopping the REPL
}

// NewGateway creates a new CLI Gateway instance.
func NewGateway(cfg *config.CLIConfig) *Gateway {
	return &Gateway{
		Cfg:    cfg,
		Reader: os.Stdin,
		Writer: os.Stdout,
	}
}

// Platform returns the platform identifier for CLI.
func (g *Gateway) Platform() domain.Platform {
	return domain.PlatformCLI
}

// Start begins the interactive REPL loop.
func (g *Gateway) Start(ctx context.Context) error {
	ctx, g.cancel = context.WithCancel(ctx)
	scanner := bufio.NewScanner(g.Reader)

	// Gate all input behind authentication before the REPL loop starts
	// (FR-001): no command or chat text is processed until login succeeds
	// or a persisted session is restored.
	if g.authHandler != nil {
		if err := g.authenticate(ctx, scanner); err != nil {
			if _, writeErr := fmt.Fprintf(g.Writer, "Authentication failed: %v\n", err); writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
			}
			return fmt.Errorf("cli gateway: authentication failed: %w", err)
		}
	}

	if g.Cfg.DebugMode {
		if _, err := fmt.Fprintln(g.Writer, "CLI Gateway started in debug mode. Type 'exit' or 'quit' to stop."); err != nil {
			// Log the error, but continue since it's a non-critical output
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}
	} else {
		if _, err := fmt.Fprintln(g.Writer, "CLI Gateway started. Type 'exit' or 'quit' to stop."); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}
	}

	for {
		// Prompt for input
		if _, err := fmt.Fprint(g.Writer, "> "); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}

		// Perform scan in a way that respects context cancellation
		scanDone := make(chan bool)
		go func() {
			scanDone <- scanner.Scan() // This can block
		}()

		select {
		case scanned := <-scanDone:
			if !scanned { // scanner.Scan() returned false
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("CLI scanner error: %w", err)
				}
				// EOF or user closed stdin
				return nil
			}
			// Process input here
			input := scanner.Text()
			input = strings.TrimSpace(input)

			if input == "" {
				continue
			}

			if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
				return nil
			}

			// Check if this is a logout command (FR-005). After logout, the
			// next command requires login again — re-run the same
			// authentication gate used at REPL start.
			if IsLogoutCommand(input) {
				if g.authHandler == nil || g.currentSession == nil {
					if _, err := fmt.Fprintln(g.Writer, "Not logged in."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}
				if err := g.authHandler.Logout(g.currentSession.ID); err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Error during logout: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				}
				g.currentUser = nil
				g.currentSession = nil
				if _, err := fmt.Fprintln(g.Writer, "Logged out."); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
				}
				if err := g.authenticate(ctx, scanner); err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Authentication failed: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
					return fmt.Errorf("cli gateway: re-authentication failed after logout: %w", err)
				}
				continue
			}

			// Check if this is a memory command
			if IsMemoryCommand(input) {
				// Memory admin commands are gated by the logged-in user's
				// real Role (FR-006/P2.6), matching the profile/bot/config
				// admin handlers' existing pattern — previously ungated,
				// relying on the now-removed always-admin auto-grant.
				// Checked before handler-availability so the permission
				// decision never depends on whether the feature happens to
				// be wired.
				if !g.hasAdminRole() {
					if _, err := fmt.Fprintln(g.Writer, "Error: insufficient permissions (admin required)."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}
				if g.memoryHandler == nil {
					if _, err := fmt.Fprintln(g.Writer, "Error: Memory commands not available."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}
				if err := g.memoryHandler.HandleMemoryCommand(ctx, input); err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				}
				continue
			}

			// Check if this is a config admin command
			if IsConfigCommand(input) {
				if g.configHandler == nil {
					if _, err := fmt.Fprintln(g.Writer, "Error: Config admin commands not available."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}

				user := g.currentUser
				if user == nil {
					user = &domain.User{
						ID:   "cli_default",
						Role: domain.RoleUser,
					}
				}

				result, err := g.configHandler.HandleConfigCommand(ctx, user, input)
				if err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				} else {
					if _, writeErr := fmt.Fprintln(g.Writer, result); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				}
				continue
			}

			// Check if this is a bot admin command
			if IsBotCommand(input) {
				if g.botHandler == nil {
					if _, err := fmt.Fprintln(g.Writer, "Error: Bot admin commands not available."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}

				// Use current user or default to a non-admin for authorization
				user := g.currentUser
				if user == nil {
					// Default CLI user (non-admin) for security
					user = &domain.User{
						ID:   "cli_default",
						Role: domain.RoleUser,
					}
				}

				// Handle bot admin command
				result, err := g.botHandler.HandleBotCommand(ctx, user, input)
				if err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				} else {
					if _, writeErr := fmt.Fprintln(g.Writer, result); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				}
				continue
			}

			// Check if this is a profile admin command
			if IsProfileCommand(input) {
				if g.profileHandler == nil {
					if _, err := fmt.Fprintln(g.Writer, "Error: Profile admin commands not available."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}

				// Use current user or default to a non-admin for authorization
				user := g.currentUser
				if user == nil {
					// Default CLI user (non-admin) for security
					user = &domain.User{
						ID:   "cli_default",
						Role: domain.RoleUser,
					}
				}

				// Handle profile admin command
				result, err := g.profileHandler.HandleProfileCommand(ctx, user, input)
				if err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				} else {
					if _, writeErr := fmt.Fprintln(g.Writer, result); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				}
				continue
			}

			// Check if this is an admin user command
			if IsAdminCommand(input) {
				if g.adminHandler == nil {
					if _, err := fmt.Fprintln(g.Writer, "Error: Admin commands not available."); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
					}
					continue
				}

				// Use current user or default to a non-admin for authorization
				user := g.currentUser
				if user == nil {
					// Default CLI user (non-admin) for security
					user = &domain.User{
						ID:   "cli_default",
						Role: domain.RoleUser,
					}
				}

				// Handle admin command
				result, err := g.adminHandler.HandleAdminCommand(ctx, user, input)
				if err != nil {
					if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				} else {
					if _, writeErr := fmt.Fprintln(g.Writer, result); writeErr != nil {
						fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
					}
				}
				continue
			}

			// Check if this is a skill command (/skill-name or /help)
			isCommand, commandName, commandArgs := parseCommand(input)
			if isCommand {
				if g.skillHandler == nil {
					// No skill handler, treat as regular message
					// Fall through to message handler
				} else {
					// Handle built-in commands
					var err error
					switch commandName {
					case "help":
						err = g.skillHandler.List(ctx)
					case "describe":
						if len(commandArgs) == 0 {
							err = fmt.Errorf("usage: /describe <skill-name>")
						} else {
							err = g.skillHandler.Describe(ctx, commandArgs[0])
						}
					default:
						// Try to execute as skill
						err = g.skillHandler.Execute(ctx, commandName, commandArgs)
					}

					if err != nil {
						if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
							fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
						}
					}
					continue
				}
			}

			if g.messageHandler == nil {
				if _, err := fmt.Fprintln(g.Writer, "Error: Message handler not set. Cannot process input."); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
				}
				continue
			}

			incomingMsg := domain.IncomingMessage{
				ID:          "cli-" + fmt.Sprintf("%d", time.Now().UnixNano()), // Unique ID
				Platform:    domain.PlatformCLI,
				PlatformUID: g.platformUID(), // AD-5: the authenticated session's Username, never a placeholder
				Text:        input,
				Timestamp:   time.Now(),
				Metadata:    nil,
			}

			if err := g.messageHandler(ctx, incomingMsg); err != nil {
				if _, writeErr := fmt.Fprintf(g.Writer, "Error processing message: %v\n", err); writeErr != nil {
					fmt.Fprintf(os.Stderr, "Error writing to CLI output (after message processing error): %v\n", writeErr)
				}
			}
		case <-ctx.Done():
			if _, err := fmt.Fprintln(g.Writer, "CLI Gateway stopping..."); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
			}
			return nil
		}
	}
}

// Stop gracefully shuts down the gateway.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.cancel != nil {
		g.cancel()
	}
	return nil
}

// Send sends a message to a user (CLI output).
func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
	_, err := fmt.Fprintf(g.Writer, "Bot: %s\n", msg.Content)
	if err != nil {
		return fmt.Errorf("failed to write to CLI output: %w", err)
	}
	return nil
}

// OnMessage registers a handler for incoming messages.
func (g *Gateway) OnMessage(handler domain.MessageHandler) {
	g.messageHandler = handler
}

// SetAdminHandler sets the admin command handler for the gateway.
func (g *Gateway) SetAdminHandler(handler *AdminCommandHandler) {
	g.adminHandler = handler
}

// SetProfileHandler sets the admin profile command handler for the gateway.
func (g *Gateway) SetProfileHandler(handler *AdminProfileCommandHandler) {
	g.profileHandler = handler
}

// SetBotHandler sets the admin bot command handler for the gateway.
func (g *Gateway) SetBotHandler(handler *AdminBotCommandHandler) {
	g.botHandler = handler
}

// SetCurrentUser sets the current user for admin command authorization.
func (g *Gateway) SetCurrentUser(user *domain.User) {
	g.currentUser = user
}

// SetMemoryHandler sets the memory command handler for the gateway.
func (g *Gateway) SetMemoryHandler(handler *MemoryCommandHandler) {
	g.memoryHandler = handler
}

// SetConfigHandler sets the admin config command handler for the gateway.
func (g *Gateway) SetConfigHandler(handler *AdminConfigCommandHandler) {
	g.configHandler = handler
}

// SetSkillHandler sets the skill command handler for the gateway.
func (g *Gateway) SetSkillHandler(handler SkillCommandHandler) {
	g.skillHandler = handler
}

// SetAuthHandler sets the login/logout command handler for the gateway
// (FR-001-FR-005). When set, Start gates all REPL input behind
// authentication (P2.1/P2.2); when nil, the gateway behaves as before this
// feature (no auth gating) — used by tests that don't exercise auth.
func (g *Gateway) SetAuthHandler(handler *AuthCommandHandler) {
	g.authHandler = handler
}

// authenticate runs the login-or-restore flow and, on success, updates
// currentUser/currentSession and propagates the authenticated identity to
// the skill handler (if it supports PlatformUIDSetter) so skill-invoked
// chat messages are attributed to the same identity as plain-text chat
// (AD-5/FR-007).
func (g *Gateway) authenticate(ctx context.Context, scanner *bufio.Scanner) error {
	user, session, err := g.authHandler.EnsureAuthenticated(ctx, g.Writer, scanner)
	if err != nil {
		return err
	}
	g.currentUser = user
	g.currentSession = session
	if setter, ok := g.skillHandler.(PlatformUIDSetter); ok {
		setter.SetPlatformUID(session.Username)
	}
	return nil
}

// platformUID returns the identifier used as domain.IncomingMessage's
// PlatformUID and, via internal/usecase/chat.Service.resolveUser, as the key
// for domain.User lookup — always the authenticated session's Username
// (AD-5's ownerUserID convention), never a hardcoded placeholder. Falls back
// to a distinct placeholder only when no auth flow is wired at all (e.g.
// tests that construct a Gateway directly without SetAuthHandler).
func (g *Gateway) platformUID() string {
	if g.currentSession != nil {
		return g.currentSession.Username
	}
	return unauthenticatedPlatformUID
}

// hasAdminRole reports whether the current CLI user (if any) has admin
// privileges. Used for command families (e.g. memory admin, P2.6) that
// don't already perform their own internal role check the way
// profile/bot/config handlers do.
func (g *Gateway) hasAdminRole() bool {
	return g.currentUser != nil && g.currentUser.Role == domain.RoleAdmin
}

// parseCommand checks if input is a command and parses it.
// Returns: isCommand, commandName, commandArgs
func parseCommand(input string) (bool, string, []string) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return false, "", nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false, "", nil
	}

	commandName := strings.TrimPrefix(parts[0], "/")
	commandArgs := parts[1:]

	return true, commandName, commandArgs
}
