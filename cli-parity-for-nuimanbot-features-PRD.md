# PRD: CLI Parity for NuimanBot Features (Chats, Projects, Jobs, Chores, History, Memories)

**Created:** 2026-08-11
**Jira:** N/A
**Status:** Draft
**Scope:** Adds real login/session identity to the existing CLI REPL (`internal/adapter/gateway/cli`), replacing today's auto-admin/no-login behavior, and adds CLI slash-commands mirroring the six web-admin environments (Chats, Projects, Jobs, Chores, History, Memories) plus the per-user half of Settings.

---

## Problem Statement

NuimanBot's six new agent-workspace environments (Chats, Projects, Jobs, Chores, History, Memories) plus Settings exist only in the web UI. The existing CLI gateway has no equivalent commands for any of them — and more fundamentally, it has no real user identity: starting the process auto-grants a hardcoded admin identity (`cli_admin`) for admin commands and a separate hardcoded `cli_user` placeholder for chat messages, with no login step at all. Since all six environments are per-user-isolated by design, CLI parity can't be added on top of that model — a real user has to be established first. This PRD closes both gaps: it adds a login step to the existing CLI REPL (reusing the web admin's existing user accounts and auth) so the session operates as a specific, authenticated user, and it adds CLI slash-commands for all six environments plus the per-user half of Settings, following the same handler pattern already used for the existing skill/profile/bot/config/memory admin commands.

## Goals

- Replace the CLI's auto-admin/no-login model with real authentication against the existing web-admin user accounts, so a CLI session operates as one specific, identified user — not an implicit trusted admin.
- Add CLI slash-commands for all six environments (Chats, Projects, Jobs, Chores, History, Memories) plus the per-user half of Settings, built on the same `internal/usecase/*` services the web UI already uses — no duplicated business logic.
- Preserve today's existing CLI capabilities (skill commands, admin bot/profile/config/memory commands, live chat) and gate them by the logged-in user's actual role, matching how the web UI already enforces admin-only vs. per-user access.

## Non-Goals

- Not building networked/concurrent multi-user CLI access (SSH multi-session, socket server) — this stays a single local terminal REPL per running process, just with real login instead of auto-admin. The current CLI has exactly one `Reader`/`Writer` per process; multiple concurrent authenticated sessions from one running instance is out of scope.
- Not changing the web UI's auth *behavior*/UX — session length, login form, password rules stay the same. The underlying credential-verification code does move to a shared usecase-layer service as part of this PRD (see FR-044), which both adapters then depend on; that's an implementation-location change, not a redesign of how web auth works or feels to a user.
- Not introducing new product surface — CLI parity mirrors the existing web FRs for the six environments; no new capabilities beyond what the web UI already has.
- Not building a CLI equivalent of the WebSocket live-update/notification-badge mechanism — that's a web-UI-specific concept; the CLI equivalent of "seeing an update" is re-running the relevant list/show command.
- Not adding account creation or password-reset flows to the CLI — it authenticates against accounts that already exist (created via the web UI's Users management), it doesn't manage account lifecycle.
- Not preserving the current no-login/always-admin behavior as a fallback mode — it is replaced, not kept alongside login as an option.

## Functional Requirements

### Authentication / Session

- **FR-001:** On REPL start, if no valid persisted session exists, prompt for username/password before allowing any command or chat input.
- **FR-002:** Login authenticates against the same user accounts/credentials the web admin uses (reuses shared credential-verification logic — see Dependencies/Risks — not a separate credential store).
- **FR-003:** On successful login, persist a session token to local disk (e.g. alongside `HistoryFile`, OS-permission-scoped) so subsequent process restarts reuse it until expiry without re-prompting.
- **FR-004:** Persisted sessions expire after a configurable duration; an expired token triggers a fresh login prompt.
- **FR-005:** A `/logout` command clears the persisted session and ends the authenticated context.
- **FR-006:** Admin-only commands (skill/profile/bot/config/memory admin, Settings' system-wide half) are gated by the logged-in user's real `Role`, replacing today's unconditional auto-admin grant.
- **FR-007:** Chat messages sent through the REPL are attributed to the logged-in user's real identity, not the hardcoded `"cli_user"` placeholder.
- **FR-044:** Credential-verification and session logic currently in `internal/adapter/web/auth.go`'s `AuthService` is extracted into a shared usecase-layer service (e.g. `internal/usecase/auth`); both the web adapter and the new CLI adapter are refactored to depend on that shared service instead of the CLI depending on the web adapter directly. The web UI's behavior/UX (session length, login form, password rules) is unchanged — only the implementation's location moves, per this repo's Clean Architecture dependency rule (adapters depend on usecase, not on each other).

### Command Surface Conventions

- **FR-008:** New environment commands follow the existing slash-command pattern (e.g. `/chat list`, `/project create`), dispatched via new `cli.New*Handler`s wired into `cliGateway` the same way existing admin handlers are (`Set*Handler`).
- **FR-009:** Command output is plain text formatted for a terminal, not JSON.
- **FR-010:** `/help` lists the new environment commands alongside existing ones; per-command usage help is available (e.g. via `/describe`).

### Chats (mirrors web FR-011–016)

- **FR-011:** `/chat list` — list the logged-in user's Chats (name, last-updated, message count).
- **FR-012:** `/chat show <id>` — display a Chat's message history.
- **FR-013:** `/chat new <first message>` — create a new Chat (auto-named from the message, same as web) and send the first message.
- **FR-014:** `/chat send <id> <message>` — send a message to an existing Chat.
- **FR-015:** `/chat delete <id>` — delete a Chat immediately.
- **FR-016:** `/chat export <id>` — export a Chat's transcript to a local file, reusing the same `ExportConversation` logic the web UI's download uses.

### Projects (mirrors FR-017–023)

- **FR-017:** `/project list` — list the logged-in user's Projects.
- **FR-018:** `/project create <name> <output-dir>` — create a Project with an output directory.
- **FR-019:** `/project show <id>` — show output directory, `AGENTS.md` presence, retention setting.
- **FR-020:** `/project chat <id> <message>` — converse with the agent in the Project's context; the primary `AGENTS.md` edit path, same as web.
- **FR-021:** `/project add-agents-file <id>` — CLI equivalent of the web's "Add AGENTS.md" control.
- **FR-022:** `/project delete <id>`.

### Jobs (mirrors FR-024–030)

- **FR-023:** `/job list [--project <id>|--chat <id>]`.
- **FR-024:** `/job create <title> <description> [--project <id>|--chat <id>]`.
- **FR-025:** `/job show <id>` — status, queue position, last run log/results/timing.
- **FR-026:** `/job chat <id> <message>` — per-Job chat interface.
- **FR-027:** `/job delete <id>`.

### Chores (mirrors FR-031–038)

- **FR-028:** `/chore list`.
- **FR-029:** `/chore create <title> <description> [--dir <path>] --schedule <cron-expr-or-preset>`.
- **FR-030:** `/chore show <id>` — schedule, last run status, run history summary.
- **FR-031:** `/chore chat <id> <message>`.
- **FR-032:** `/chore delete <id>` — soft-delete, same semantics as the web UI's Chore/Job delete parity fix.
- **FR-033:** `/chore confirm-schedule <id>` — approves an agent-proposed schedule, mirroring the web UI's confirmation requirement.

### History (mirrors FR-040–044)

- **FR-034:** `/history list [--job <id>|--chore <id>] [--status <status>] [--since <date>]`.
- **FR-035:** `/history show <run-id>` — log, results, timing.
- **FR-036:** `/history chat <run-id> <message>` — ask the agent about a specific run.

### Memories (mirrors FR-045–047)

- **FR-037:** `/memories browse [query]` — read-only search/browse over the agent-maintained memory store. Uses the `/memories` prefix (plural), distinct from the existing `/memory` admin-command prefix (stats/export/import/rebuild-FTS) to avoid collision.
- **FR-038:** `/memories chat <cell-id> <message>` — discuss or request edits to a memory entry; the agent remains the sole writer, same as web.

### Settings (mirrors FR-001–004, split by scope)

- **FR-039:** `/settings show` — per-user settings: Chat/Project/History retention values, including "Never".
- **FR-040:** `/settings set retention <chat|project|history> <value|never>` — per-user.
- **FR-041:** `/settings show --system` (admin-only) — worker pool size, network access mode/allowlist.
- **FR-042:** `/settings set worker-pool-size <n>` (admin-only).
- **FR-043:** `/settings set network-mode <localhost|remote>` (admin-only) — inherits the same known limitation as the web UI (doesn't actually rebind the running listener); the CLI command's help text must say so, not overpromise.

## Non-Functional Requirements

- **Performance:** List commands (`/history list`, `/chat list`, etc.) paginate or truncate for large result sets rather than dumping unbounded output to the terminal.
- **Reliability:** A missing or corrupted session-token file falls back to a fresh login prompt rather than crashing the REPL.
- **Security:** Session tokens are stored on disk with restrictive OS permissions (e.g. `0600`) and are never logged in plaintext; failed login attempts don't leak whether a username exists vs. a wrong password. Admin-gating (FR-006) and per-user data isolation are enforced by calling the same `internal/usecase/*` methods the web adapter uses (`ownerUserID` scoping) — the CLI must not reimplement isolation checks separately, avoiding a second, potentially weaker path to the same data.
- **Observability:** Failed login attempts are logged/audited, consistent with the web admin's existing login rate-limiter and audit logging.

## Acceptance Criteria

- [ ] Starting the CLI with no valid session prompts for username/password before any command or chat input is accepted.
- [ ] A correct login persists a session token to disk; restarting the process within the expiry window does not re-prompt.
- [ ] An expired or missing/corrupted token triggers a fresh login prompt without crashing.
- [ ] `/logout` clears the persisted session; the next command requires login again.
- [ ] A non-admin user's attempt to run an admin-only command (e.g. `/settings set worker-pool-size`) is rejected with a clear permission error, not silently allowed.
- [ ] A chat message sent via the REPL is attributed to the logged-in user's real ID, verifiable in that Chat's stored history.
- [ ] `/chat`, `/project`, `/job`, `/chore`, `/history`, `/memories` commands each round-trip against the same usecase services the web UI uses — data created via CLI is visible in the web UI and vice versa.
- [ ] Two different logged-in users each see only their own Chats/Projects/Jobs/Chores/History via CLI commands — no cross-user visibility.
- [ ] `/memories browse` and `/memory stats` (existing admin command) both work without prefix collision.
- [ ] All existing pre-change CLI behavior (skill commands, admin bot/profile/config/memory commands minus the auto-admin grant) continues to work for a logged-in admin user.
- [ ] `internal/adapter/cli` does not import `internal/adapter/web` anywhere — both adapters depend only on the new shared usecase-layer auth service (FR-044).
- [ ] The web admin's login behavior (session length, login form, password rules) is unchanged after the `AuthService` extraction — verified by the existing web auth tests still passing unmodified.
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds and the quality-gate chain from `AGENTS.md` passes.

## Dependencies and Risks

| Item | Type | Notes |
|---|---|---|
| `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` | Dependency | Existing per-user-scoped business logic the new CLI handlers call directly — no duplicated logic. |
| `internal/adapter/web/auth.go`'s `AuthService` | Risk | Currently adapter-local (web-only), not usecase-layer — reusing it as-is from a CLI adapter would violate this repo's Clean Architecture dependency rule (adapters depend on usecase, not on each other). FR-044 mandates the extraction into a shared usecase-layer service both adapters depend on — real, non-trivial refactoring, not a drop-in reuse. |
| `IsMemoryCommand()` prefix routing in `cliGateway` | Dependency/Risk | New `/memories` commands must be dispatched distinctly from the existing `/memory` admin-command prefix; incorrect parsing could let one silently shadow the other. |
| FR-R7 (identity-bridge gap, from the prior feature's code review) | Dependency | This PRD is effectively the fix for FR-R7 — Memories' `ownerUserID`→`ConversationID` scoping assumption becomes verifiable/real once CLI sessions carry a real logged-in identity. |
| Removing the auto-admin grant | Risk | `main.go`'s `SetCurrentUser(&domain.User{Role: RoleAdmin})` is currently unconditional; anyone relying on today's zero-friction admin CLI access (local scripts, ops runbooks) breaks once login is required — needs a deprecation note in `support_docs/`. |
| Settings' network-mode limitation (known, pre-existing) | Risk | `/settings set network-mode` inherits the same gap as the web UI — doesn't actually rebind the listener. The CLI's command description must not overpromise what the web UI already admits it can't do. |

## Open Questions

- Session-token expiry duration is not pinned — propose a sensible default (e.g. matching or complementing the web admin's session expiry) during spec/implementation planning.
- Exact CLI login UX for password entry (masked terminal input vs. plain) is left as an implementation detail for the spec phase.
- Whether `/settings set network-mode` from the CLI should be blocked entirely (admin must use the web UI) rather than exposed with a caveat, given it can't fully apply from either surface today — worth revisiting once/if the underlying web Settings limitation (FR-R11) is fixed in a future pass.
