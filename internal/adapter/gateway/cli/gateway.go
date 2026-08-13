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

// EnvCommandHandler is the common interface for the per-user environment
// command families this feature adds — Chats (FR-011-016), Projects
// (FR-017-022), Jobs (FR-023-027), Chores (FR-028-033), History
// (FR-034-036), Memories (FR-037-038), and Settings (FR-039-043) — following
// the existing AdminXCommandHandler pattern (HandleXCommand(ctx,
// currentUser, input) (string, error)), but additionally receiving
// ownerUserID as an explicit parameter distinct from currentUser: AD-5
// requires every internal/usecase/{chats,projects,jobs,chores,history,
// memories} call to use the authenticated session's Username specifically,
// never currentUser.ID or any other domain.User field, and passing it
// separately removes the guesswork rather than relying on each handler to
// pick the right field off currentUser.
type EnvCommandHandler interface {
	// Handle processes input (the full command line, including its "/prefix"
	// token) for the given currentUser (role/permission checks) and
	// ownerUserID (AD-5's per-user data-scoping key — always
	// currentSession.Username), returning the formatted terminal response or
	// an error.
	Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error)
}

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

	// Per-user environment command handlers (Phase C/D). Each is nil until
	// wired via its Set*Handler; dispatchEnvCommand shows a "not available"
	// message for a nil handler rather than failing dispatch.
	chatsHandler    EnvCommandHandler
	projectsHandler EnvCommandHandler
	jobsHandler     EnvCommandHandler
	choresHandler   EnvCommandHandler
	historyHandler  EnvCommandHandler
	memoriesHandler EnvCommandHandler // distinct from memoryHandler above: "/memories" (plural, FR-037/038) vs "/memory " (singular, admin) — see AD-3
	settingsHandler EnvCommandHandler
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

			// Environment command families (Phase C/D): Chats, Projects,
			// Jobs, Chores, History, Memories, Settings. Available to any
			// authenticated user (not admin-gated) — each handler scopes
			// data internally via ownerUserID (AD-5). Checked in this order
			// so that "/memories ..." (plural) never falls into the
			// existing "/memory " (singular, admin) branch above or vice
			// versa (AD-3 — the two prefixes are verified non-colliding,
			// see isEnvCommand's doc comment).
			if IsChatCommand(input) {
				g.dispatchEnvCommand(ctx, g.chatsHandler, "Chat", input)
				continue
			}
			if IsProjectCommand(input) {
				g.dispatchEnvCommand(ctx, g.projectsHandler, "Project", input)
				continue
			}
			if IsJobCommand(input) {
				g.dispatchEnvCommand(ctx, g.jobsHandler, "Job", input)
				continue
			}
			if IsChoreCommand(input) {
				g.dispatchEnvCommand(ctx, g.choresHandler, "Chore", input)
				continue
			}
			if IsHistoryCommand(input) {
				g.dispatchEnvCommand(ctx, g.historyHandler, "History", input)
				continue
			}
			if IsMemoriesCommand(input) {
				g.dispatchEnvCommand(ctx, g.memoriesHandler, "Memories", input)
				continue
			}
			if IsSettingsCommand(input) {
				g.dispatchEnvCommand(ctx, g.settingsHandler, "Settings", input)
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

// SetChatsHandler sets the Chats environment command handler (FR-011-016).
func (g *Gateway) SetChatsHandler(handler EnvCommandHandler) {
	g.chatsHandler = handler
}

// SetProjectsHandler sets the Projects environment command handler (FR-017-022).
func (g *Gateway) SetProjectsHandler(handler EnvCommandHandler) {
	g.projectsHandler = handler
}

// SetJobsHandler sets the Jobs environment command handler (FR-023-027).
func (g *Gateway) SetJobsHandler(handler EnvCommandHandler) {
	g.jobsHandler = handler
}

// SetChoresHandler sets the Chores environment command handler (FR-028-033).
func (g *Gateway) SetChoresHandler(handler EnvCommandHandler) {
	g.choresHandler = handler
}

// SetHistoryHandler sets the History environment command handler (FR-034-036).
func (g *Gateway) SetHistoryHandler(handler EnvCommandHandler) {
	g.historyHandler = handler
}

// SetMemoriesHandler sets the Memories environment command handler
// (FR-037-038, "/memories" plural — distinct from SetMemoryHandler's
// "/memory " singular admin commands).
func (g *Gateway) SetMemoriesHandler(handler EnvCommandHandler) {
	g.memoriesHandler = handler
}

// SetSettingsHandler sets the Settings environment command handler (FR-039-043).
func (g *Gateway) SetSettingsHandler(handler EnvCommandHandler) {
	g.settingsHandler = handler
}

// dispatchEnvCommand routes input to handler (one of the seven per-user/
// Settings environment command families) and writes its formatted result or
// error to g.Writer. A nil handler (not yet wired) produces a clear
// "commands not available" message rather than falling through to the
// message handler or panicking. Mirrors the defensive
// nil-currentUser-defaults-to-non-admin pattern the pre-existing admin
// command dispatch (IsAdminCommand et al.) already uses.
func (g *Gateway) dispatchEnvCommand(ctx context.Context, handler EnvCommandHandler, label, input string) {
	if handler == nil {
		if _, err := fmt.Fprintf(g.Writer, "Error: %s commands not available.\n", label); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}
		return
	}

	user := g.currentUser
	if user == nil {
		user = &domain.User{ID: "cli_default", Role: domain.RoleUser}
	}

	result, err := handler.Handle(ctx, user, g.platformUID(), input)
	if err != nil {
		if _, writeErr := fmt.Fprintf(g.Writer, "Error: %v\n", err); writeErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
		}
		return
	}
	if _, writeErr := fmt.Fprintln(g.Writer, result); writeErr != nil {
		fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", writeErr)
	}
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

// isEnvCommand reports whether input's first token is exactly prefix — i.e.
// input equals prefix (bare command, no args — routes to the handler's own
// help output rather than falling through as unrecognized) or begins with
// "prefix " (a genuine word-boundary match, never a longer command word that
// merely starts with the same characters). This is the same discipline
// IsMemoryCommand already follows for "/memory " (note trailing space); it
// is what keeps "/memories ..." (plural, IsMemoriesCommand) from ever
// colliding with "/memory " (singular, admin) or vice versa — AD-3,
// verified in architecture.md's Integration Points section: the two
// prefixes diverge at their 7th character ('y' vs 'i'), well before either
// reaches a space.
func isEnvCommand(input, prefix string) bool {
	return input == prefix || strings.HasPrefix(input, prefix+" ")
}

// IsChatCommand checks if the input is a Chats environment command (FR-011-016).
func IsChatCommand(input string) bool { return isEnvCommand(input, "/chat") }

// IsProjectCommand checks if the input is a Projects environment command (FR-017-022).
func IsProjectCommand(input string) bool { return isEnvCommand(input, "/project") }

// IsJobCommand checks if the input is a Jobs environment command (FR-023-027).
func IsJobCommand(input string) bool { return isEnvCommand(input, "/job") }

// IsChoreCommand checks if the input is a Chores environment command (FR-028-033).
func IsChoreCommand(input string) bool { return isEnvCommand(input, "/chore") }

// IsHistoryCommand checks if the input is a History environment command (FR-034-036).
func IsHistoryCommand(input string) bool { return isEnvCommand(input, "/history") }

// IsMemoriesCommand checks if the input is a Memories environment command
// (FR-037-038, plural "/memories" — distinct from IsMemoryCommand's
// singular "/memory ", see isEnvCommand's doc comment for the AD-3
// non-collision argument).
func IsMemoriesCommand(input string) bool { return isEnvCommand(input, "/memories") }

// IsSettingsCommand checks if the input is a Settings environment command (FR-039-043).
func IsSettingsCommand(input string) bool { return isEnvCommand(input, "/settings") }
