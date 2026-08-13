# Spec: CLI Parity for NuimanBot Features (Chats, Projects, Jobs, Chores, History, Memories)

**Created:** 2026-08-11
**Source PRD:** [cli-parity-for-nuimanbot-features-PRD.md](./cli-parity-for-nuimanbot-features-PRD.md)
**Status:** Reviewed (pipeline Step 2, `dev-flow:review-spec`) — implementation-ready. FR-020, FR-026, FR-031, FR-036 marked DEFERRED (no backing web capability; see Non-Goals and `architecture.md`'s "Scope correction"). AD-1/AD-2 in `architecture.md` were revised against the actual code, not just reviewed as drafted — see that file's "Review Findings (Step 2)" section for what changed.

## Executive Summary

NuimanBot's six agent-workspace environments (Chats, Projects, Jobs, Chores, History, Memories) plus per-user Settings currently exist only in the web UI. This spec adds real login/session identity to the existing CLI REPL (`internal/adapter/gateway/cli`) — replacing today's auto-admin/no-login startup behavior — and adds CLI slash-commands mirroring all six environments plus the per-user half of Settings, built on the same `internal/usecase/*` services the web UI already uses. The highest-risk element is FR-044: extracting `AuthService` out of `internal/adapter/web/auth.go` into a shared usecase-layer service so both the web and CLI adapters can depend on it without violating this repo's Clean Architecture dependency rule (adapters must not depend on other adapters).

## Problem Statement

The CLI gateway has no equivalent commands for any of the six new web-admin environments, and more fundamentally has no real user identity: starting the process auto-grants a hardcoded admin identity (`cli_admin`, `RoleAdmin`) for admin commands via an unconditional `cliGateway.SetCurrentUser(...)` call in `cmd/nuimanbot/main.go`, and a separate hardcoded `"cli_user"` placeholder is used for chat message attribution — with no login step at all. Since all six environments are per-user-isolated by design, CLI parity cannot be layered on top of that model — a real, authenticated user identity has to be established first.

Confirmed in the current repo (`worktree-cli-parity`, rooted on `main` post-PR #8):
- `cmd/nuimanbot/main.go:1156-1159` — unconditional `cliGateway.SetCurrentUser(&domain.User{ID: "cli_admin", Username: "cli_administrator", Role: domain.RoleAdmin})`.
- `cmd/nuimanbot/main.go:1173` — `skillHandler.SetMessageHandler(messageHandler, domain.PlatformCLI, "cli_user")` hardcodes chat attribution.
- `internal/adapter/web/auth.go` — `AuthService` (`ValidateCredentials`, `CreateSession`, `ValidateSession`, `GetSession`, `DestroySession`, session-cleanup loop, CSRF, etc.) is defined entirely in the `web` adapter package — not in `internal/usecase`.
- `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` already exist and back the web UI's six environments plus Settings.
- `internal/adapter/gateway/cli/gateway.go` already has a `Set*Handler` wiring pattern (`SetAdminHandler`, `SetProfileHandler`, `SetBotHandler`, `SetMemoryHandler`, `SetConfigHandler`, `SetSkillHandler`) that new environment handlers should follow.
- `internal/adapter/gateway/cli/memory_commands.go` — `IsMemoryCommand()` already claims the `/memory` prefix for existing admin commands (stats/export/import/rebuild-FTS); new commands must use `/memories` (plural) per FR-037/FR-038 to avoid prefix collision.

## Goals

- Replace the CLI's auto-admin/no-login model with real authentication against the existing web-admin user accounts, so a CLI session operates as one specific, identified user — not an implicit trusted admin.
- Add CLI slash-commands for all six environments (Chats, Projects, Jobs, Chores, History, Memories) plus the per-user half of Settings, built on the same `internal/usecase/*` services the web UI already uses — no duplicated business logic.
- Preserve today's existing CLI capabilities (skill commands, admin bot/profile/config/memory commands, live chat) and gate them by the logged-in user's actual role, matching how the web UI already enforces admin-only vs. per-user access.

## Non-Goals

- Networked/concurrent multi-user CLI access (SSH multi-session, socket server) — stays a single local terminal REPL per running process, just with real login instead of auto-admin.
- Changing the web UI's auth *behavior*/UX — session length, login form, password rules stay the same. `AuthService` moves location (FR-044) but is not redesigned.
- New product surface beyond what the web UI already has for the six environments.
- A CLI equivalent of the WebSocket live-update/notification-badge mechanism — the CLI equivalent of "seeing an update" is re-running the relevant list/show command.
- Account creation or password-reset flows in the CLI — it authenticates against accounts already created via the web UI's Users management.
- Preserving the current no-login/always-admin behavior as a fallback mode — it is replaced, not kept alongside login as an option.
- **Per-item "converse with the agent" chat sub-commands for Projects, Jobs, Chores, and History** (originally drafted as FR-020/FR-026/FR-031/FR-036) — added as a Non-Goal during Step 2 review after verifying against the current merged web code (`internal/adapter/web/{projects,jobs,chores,history}_handler.go`) that none of these four environments' web-side `*Service` interfaces has a chat/converse method today. Three of the four handler files explicitly document this as their own already-decided scope cut (`jobs_handler.go:155`, `chores_handler.go:102`, `history_handler.go:230`). Building CLI-only chat capability with no web-side counterpart would itself violate this PRD's "no new capabilities beyond what the web UI already has" Non-Goal — so these four FRs are marked DEFERRED below rather than implemented. `/memories chat` (FR-038) is unaffected — it has a real backing method (`AskAboutCell`).

## User Requirements / Functional Requirements

### Authentication / Session

- **FR-001:** On REPL start, if no valid persisted session exists, prompt for username/password before allowing any command or chat input.
- **FR-002:** Login authenticates against the same user accounts/credentials the web admin uses (reuses shared credential-verification logic — not a separate credential store).
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
- **FR-020 (DEFERRED — see Step 2 review note below):** `/project chat <id> <message>` — converse with the agent in the Project's context. **Descoped from this pass**: verified against the current merged web code that `ProjectsService` (`internal/adapter/web/projects_handler.go`) has no chat/converse method — there is no web-side capability to mirror. Building one here would violate this PRD's own Non-Goal ("no new capabilities beyond what the web UI already has"). Revisit once the web UI ships this. `/project add-agents-file` (FR-021) remains the CLI's `AGENTS.md`-editing path for this pass.
- **FR-021:** `/project add-agents-file <id>` — CLI equivalent of the web's "Add AGENTS.md" control.
- **FR-022:** `/project delete <id>`.

### Jobs (mirrors FR-024–030)

- **FR-023:** `/job list [--project <id>|--chat <id>]`.
- **FR-024:** `/job create <title> <description> [--project <id>|--chat <id>]`.
- **FR-025:** `/job show <id>` — status, queue position, last run log/results/timing.
- **FR-026 (DEFERRED — see Step 2 review note below):** `/job chat <id> <message>` — per-Job chat interface. **Descoped from this pass**: `internal/adapter/web/jobs_handler.go:155` explicitly documents this as "out of scope for this pass" on the web side too (comment references its own FR-029). No `JobsService` method exists to mirror.
- **FR-027:** `/job delete <id>`.

### Chores (mirrors FR-031–038)

- **FR-028:** `/chore list`.
- **FR-029:** `/chore create <title> <description> [--dir <path>] --schedule <cron-expr-or-preset>`.
- **FR-030:** `/chore show <id>` — schedule, last run status, run history summary.
- **FR-031 (DEFERRED — see Step 2 review note below):** `/chore chat <id> <message>`. **Descoped from this pass**: `internal/adapter/web/chores_handler.go:102` documents a chat interface exists conceptually but "not this form" — not implemented in `ChoresService`. No web-side capability to mirror.
- **FR-032:** `/chore delete <id>` — soft-delete, same semantics as the web UI's Chore/Job delete parity fix.
- **FR-033:** `/chore confirm-schedule <id>` — approves an agent-proposed schedule, mirroring the web UI's confirmation requirement.

### History (mirrors FR-040–044)

- **FR-034:** `/history list [--job <id>|--chore <id>] [--status <status>] [--since <date>]`.
- **FR-035:** `/history show <run-id>` — log, results, timing.
- **FR-036 (DEFERRED — see Step 2 review note below):** `/history chat <run-id> <message>` — ask the agent about a specific run. **Descoped from this pass**: `internal/adapter/web/history_handler.go:230` explicitly documents "per-run chat interface is out of scope for this environment." No `HistoryService` method exists to mirror.

### Memories (mirrors FR-045–047)

- **FR-037:** `/memories browse [query]` — read-only search/browse over the agent-maintained memory store. Uses the `/memories` prefix (plural), distinct from the existing `/memory` admin-command prefix (stats/export/import/rebuild-FTS) to avoid collision.
- **FR-038:** `/memories chat <cell-id> <message>` — discuss or request edits to a memory entry; the agent remains the sole writer, same as web.

### Settings (mirrors FR-001–004, split by scope)

- **FR-039 (partially DEFERRED — found during Phase C/D implementation, same class of gap as FR-020/026/031/036):** `/settings show` — displays the **system-wide** Chat/Project/History retention defaults (including "Never"), read-only. The originally-specified "per-user settings" framing is not implementable as written: verified against `internal/domain/preferences.go`'s `UserPreferences` (the only per-user settings store that exists) and `internal/adapter/web/settings_handler.go`'s `SettingsService` interface doc comment, which states outright that "per-user retention override storage (beyond displaying the system-wide default) is deferred." There is no per-user retention override anywhere in the web UI to mirror; `RetentionPolicy` is fixed per-resource at creation time from a single system-wide default. Building a real per-user override now would invent new product surface this PRD's own Non-Goal excludes ("no new capabilities beyond what the web UI already has"), and isn't CLI-only work — it would touch chat/project/history usecase creation paths too. `/settings show` therefore mirrors exactly what the web UI's Settings page shows today.
- **FR-040 (DEFERRED — same reasoning as FR-039 above):** `/settings set retention <chat|project|history> <value|never>` — per-user. No backing capability exists to mirror. A stub/clear "not yet implemented — no per-user retention override exists in the web UI to mirror" response is fine if invoked, matching the FR-020/026/031/036 convention.
- **FR-041 (partially DEFERRED — see FR-043):** `/settings show --system` (admin-only) — worker pool size (real) and skills count (real) display; network access mode display is DEFERRED along with FR-043 below (no shared state to read).
- **FR-042:** `/settings set worker-pool-size <n>` (admin-only). Fully backed by `internal/usecase/settings.Service`, implemented as specified.
- **FR-043 (DEFERRED — found during Phase C/D implementation; this PRD's own Open Questions section already flagged the doubt, see below):** `/settings set network-mode <localhost|remote>` (admin-only). Verified during implementation: `domain.NetworkAccessConfig` is owned entirely as private state inside `*web.Server` (`internal/adapter/web/middleware.go`'s `networkAccessState`), with no shared usecase-layer holder the CLI can read or mutate without importing `internal/adapter/web` (a hard acceptance-criterion violation). This PRD's own Open Questions section had already flagged this exact doubt: *"Whether `/settings set network-mode` from the CLI should be blocked entirely... given it can't fully apply from either surface today — worth revisiting once/if the underlying web Settings limitation (FR-R11) is fixed in a future pass."* FR-R11 (from the prior six-environments feature) is the root blocker: even the web UI's own network-mode setting doesn't rebind the running listener. Sharing state between CLI and web would make the value consistent between adapters but still not functionally effective — not worth an unplanned refactor (`web/middleware.go`, `web/server.go`, `settings.Service`, `main.go` wiring) until FR-R11 itself is fixed. A stub/clear "not yet implemented — deferred until FR-R11 is addressed" response is fine if invoked, matching the FR-020/026/031/036/040 convention. Revisit FR-043 together with FR-R11 in a future pass, not independently.

## Non-Functional Requirements

- **Performance:** List commands (`/history list`, `/chat list`, etc.) paginate or truncate for large result sets rather than dumping unbounded output to the terminal.
- **Reliability:** A missing or corrupted session-token file falls back to a fresh login prompt rather than crashing the REPL.
- **Security:** Session tokens are stored on disk with restrictive OS permissions (e.g. `0600`) and are never logged in plaintext; failed login attempts don't leak whether a username exists vs. a wrong password. Admin-gating (FR-006) and per-user data isolation are enforced by calling the same `internal/usecase/*` methods the web adapter uses (`ownerUserID` scoping) — the CLI must not reimplement isolation checks separately.
- **Observability:** Failed login attempts are logged/audited, consistent with the web admin's existing login rate-limiter and audit logging. **Correction (CLI-parity auto-review fix pass, FR-005, 2026-08-11):** verified against the actual code — the web admin's login rate limiter (`loginRateLimiterStore`, 5 attempts/minute) is real, but its "audit logging" is not: `internal/adapter/web/auth.go`'s `handleLogin` does not call into `internal/infrastructure/audit` (the `AuditLogger` type exists in the codebase but is unused there), and neither did this feature's own `internal/adapter/gateway/cli/auth_commands.go`. So "consistent with the web admin's existing... audit logging" accurately describes "consistent with the web admin's existing lack of failed-login audit logging" — this was a pre-existing gap this feature did not introduce and did not close, not a regression. Tracked as a real follow-up: [stainedhead/NuimanBot#9](https://github.com/stainedhead/NuimanBot/issues/9) (one combined issue covering both `web/auth.go` and `cli/auth_commands.go`, since they share no code path apart from the `audit` package itself and the fix shape is identical in both places).

## System Architecture

**Affected layers:**
- `internal/usecase/auth` (new) — shared credential-verification and session-management service extracted from `internal/adapter/web/auth.go`, per FR-044. See `architecture.md` for the extraction design — this is the PRD's highest architectural risk and must be settled before implementation begins.
- `internal/adapter/web` — refactored to depend on `internal/usecase/auth` instead of owning `AuthService` directly; behavior/UX unchanged.
- `internal/adapter/gateway/cli` — new: login/session handling, `/logout`, new `cli.New*Handler`s for Chats/Projects/Jobs/Chores/History/Memories/Settings, wired via new `Set*Handler` methods on `Gateway`, following the existing pattern.
- `cmd/nuimanbot/main.go` — `SetCurrentUser` auto-admin call removed/replaced with a login flow at REPL start; `"cli_user"` hardcoded chat attribution replaced with the logged-in user's real ID.

**New/modified components:** see `architecture.md` and `data-dictionary.md` for full detail — populated in the architecture phase of this spec.

## Scope of Changes

**Files likely to be created:**
- `internal/usecase/auth/` (service, interfaces, tests)
- `internal/adapter/gateway/cli/auth_commands.go` (or similar) — login/logout REPL flow
- `internal/adapter/gateway/cli/chat_commands.go`, `project_commands.go`, `job_commands.go`, `chore_commands.go`, `history_commands.go`, `memories_commands.go`, `settings_commands.go` (plus corresponding `_test.go`)

**Files likely to be modified:**
- `internal/adapter/web/auth.go` — refactored to delegate to `internal/usecase/auth`
- `internal/adapter/gateway/cli/gateway.go` — new `Set*Handler` methods, dispatch routing, session-gating on existing admin commands
- `cmd/nuimanbot/main.go` — remove auto-admin `SetCurrentUser` call, wire login flow and new handlers

**Dependencies:** `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` (existing, reused as-is per Non-Goals).

## Breaking Changes

- **Behavioral:** Running `./bin/nuimanbot` no longer grants immediate admin access — a login prompt is required. This is an intentional, PRD-mandated breaking change (see PRD Non-Goals: "Not preserving the current no-login/always-admin behavior as a fallback mode"). Needs a deprecation note in `support_docs/` per the PRD's Dependencies/Risks table, since local scripts/ops runbooks relying on today's zero-friction admin CLI access will break.
- **No API/schema breaking changes** — `internal/usecase/*` interfaces are consumed, not changed. `AuthService`'s public surface should be preserved in its new location so `internal/adapter/web` callers require minimal changes.

## Success Criteria and Acceptance Criteria

- [ ] Starting the CLI with no valid session prompts for username/password before any command or chat input is accepted.
- [ ] A correct login persists a session token to disk; restarting the process within the expiry window does not re-prompt.
- [ ] An expired or missing/corrupted token triggers a fresh login prompt without crashing.
- [ ] `/logout` clears the persisted session; the next command requires login again.
- [ ] A non-admin user's attempt to run an admin-only command (e.g. `/settings set worker-pool-size`) is rejected with a clear permission error, not silently allowed.
- [ ] A chat message sent via the REPL is attributed to the logged-in user's real ID, verifiable in that Chat's stored history.
- [ ] `/chat`, `/project`, `/job`, `/chore`, `/history`, `/memories` commands each round-trip against the same usecase services the web UI uses, for the sub-commands actually implemented this pass (see Non-Goals: Project/Job/Chore/History per-item chat sub-commands are deferred, not built) — data created via CLI is visible in the web UI and vice versa.
- [ ] Two different logged-in users each see only their own Chats/Projects/Jobs/Chores/History via CLI commands — no cross-user visibility. Includes: a `/job create --project <id>` (or `--chat <id>`) where `<id>` belongs to a Project/Chat the logged-in user does not own returns a not-found/permission error, never silently attaches to another user's Project/Chat. (This is already enforced by construction as long as the CLI handler always passes the *logged-in* user's `Username` as `ownerUserID` — see `architecture.md` AD-5 and `internal/usecase/jobs/service.go`'s `ProjectDirectoryLookup.OutputDirectoryFor(ctx, ownerUserID, projectID)`, which is itself scoped by the caller-supplied `ownerUserID`, not by the target project's actual owner. Needs an explicit acceptance test, not just reliance on the underlying scoping.)
- [ ] `/memories browse` and `/memory stats` (existing admin command) both work without prefix collision; bare `/memories` with no arguments shows help (FR-010) rather than falling through unrecognized.
- [ ] All existing pre-change CLI behavior (skill commands, admin bot/profile/config/memory commands minus the auto-admin grant) continues to work for a logged-in admin user.
- [ ] A non-admin CLI user's first plain-text chat message does not silently grant them RBAC admin tool-execution privileges via `internal/usecase/chat`'s `defaultRoleForPlatform(PlatformCLI)` auto-provisioning path — verified per `architecture.md` AD-6's identity-reconciliation step.
- [ ] `internal/adapter/gateway/cli` (the package this feature actually modifies — corrected during Step 2 review from an earlier draft's incorrect `internal/adapter/cli`, an unrelated pre-existing package with no auth/login concerns) does not import `internal/adapter/web` anywhere — both adapters depend only on the new shared usecase-layer auth service (FR-044).
- [ ] The web admin's login behavior (session length, login form, password rules) is unchanged after the `AuthService` extraction — verified by the existing web auth tests still passing unmodified, with the sole documented exception of `TestSessionExpiry` and `TestCleanupExpiredSessions` relocating (test bodies unchanged) into `internal/usecase/auth` per `architecture.md` AD-1.
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds and the quality-gate chain from `AGENTS.md` passes (fmt, tidy, vet, lint, test, build, run).

## Risks and Mitigation

| Risk | Notes | Mitigation |
|---|---|---|
| `AuthService` extraction (FR-044) | Currently adapter-local (web-only); highest architectural risk in this PRD — real refactoring, not drop-in reuse. | Settle extraction design in `architecture.md` before any implementation task begins; verify with existing web auth tests unmodified. |
| `IsMemoryCommand()` prefix routing | New `/memories` commands must be dispatched distinctly from existing `/memory` admin prefix; incorrect parsing could let one shadow the other. | Explicit prefix-disambiguation logic + tests covering both prefixes side by side. |
| FR-R7 (identity-bridge gap, from prior feature's code review) | This PRD is effectively the fix for FR-R7 — Memories' `ownerUserID`→`ConversationID` scoping assumption becomes verifiable only once CLI sessions carry a real logged-in identity. | Verify with an acceptance test once real CLI identity exists. |
| Removing the auto-admin grant | Anyone relying on today's zero-friction admin CLI access (local scripts, ops runbooks) breaks once login is required. | Deprecation note in `support_docs/`, called out in release notes. |
| Settings' network-mode limitation (known, pre-existing) | `/settings set network-mode` inherits the same gap as web UI — doesn't rebind the listener. | CLI help text must state the limitation explicitly, matching web UI's existing admission. |
| `internal/usecase/chat`'s `defaultRoleForPlatform(PlatformCLI) = RoleAdmin` (found during Step 2 review) | A second, independent "CLI = trusted = admin" shortcut in the existing RBAC/chat-message engine, separate from `main.go`'s auto-admin `SetCurrentUser` call. Not addressed by anything else in this spec by default — would silently grant every CLI user RBAC admin tool-execution privileges on their first chat message if left as-is. | CLI login flow reconciles `domain.User`'s `Role` for `(PlatformCLI, session.Username)` immediately post-login, per `architecture.md` AD-6, before any chat input is accepted. |
| Type-alias approach for `AuthService` extraction does not compile (found during Step 2 review) | The original AD-1 draft (`type AuthService = auth.Service`) breaks `internal/adapter/web/auth_coverage_test.go`'s white-box tests and two production call sites, because a type alias cannot expose another package's unexported members. | Wrapper-struct approach (embeds `*auth.Service`, keeps CSRF/cookie fields and thin unexported shims in `web`) — see `architecture.md` AD-1, revised. |
| Three of five per-item chat sub-commands have no web-side capability to mirror (found during Step 2 review) | FR-020/FR-026/FR-031/FR-036 as originally drafted assumed web parity that doesn't exist; building them would violate this PRD's own Non-Goal. | Deferred — see Non-Goals and the corresponding FR entries below, `architecture.md`'s "Scope correction" section. |

## Timeline and Milestones

[TBD — to be defined in `plan.md` during the planning phase.]

## References

- Source PRD: [specs/260811-cli-parity-for-nuimanbot-features/cli-parity-for-nuimanbot-features-PRD.md](./cli-parity-for-nuimanbot-features-PRD.md)
- `internal/adapter/web/auth.go` — current `AuthService` implementation (extraction source)
- `internal/adapter/gateway/cli/gateway.go` — existing CLI dispatch/handler-wiring pattern
- `internal/adapter/gateway/cli/memory_commands.go` — `IsMemoryCommand()` prefix logic
- `cmd/nuimanbot/main.go` — current auto-admin `SetCurrentUser` call site (lines ~1156-1173)
- PR #8 — merged six new web-admin environments this PRD mirrors in the CLI
