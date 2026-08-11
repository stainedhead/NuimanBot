# Tasks: CLI Parity for NuimanBot Features

**Feature:** cli-parity-for-nuimanbot-features
**Created:** 2026-08-11
**Status:** Task breakdown complete (finalized during pipeline Step 2, `dev-flow:review-spec`) — ready for implementation (pipeline Step 3+).

## Progress Summary

0/19 tasks complete. Full breakdown below, organized by `plan.md`'s Phase A–E structure. TDD (Red-Green-Refactor) applies to every task per `AGENTS.md` — "acceptance criteria" below are the Green-phase bar, not a substitute for writing the failing test first.

## Phase A: AuthService Extraction (FR-044) — blocks everything else

- **P1.1 — Extract `internal/usecase/auth` package (wrapper-struct approach, AD-1)**
  - **Dependencies:** None (first task in the critical path per `plan.md`)
  - **Description:** Move `AuthService`/`AuthUser`/`Session` and credential/session methods from `internal/adapter/web/auth.go` into a new `internal/usecase/auth` package per `architecture.md` AD-1 (revised: wrapper struct, not a type alias). Export `CreateSessionWithFlags`, `IsDefaultCredentials`; add new `GetUser(username string) (AuthUser, bool)`. Refactor `internal/adapter/web.AuthService` into a wrapper embedding `*auth.Service`, keeping `csrfTokens`/`secureCookies` and the four unexported shim methods (`setSecureCookies`/`isSecureCookies`/`createSessionWithFlags`/`isDefaultCredentials`) in `web`. Fix the `s.auth.users[username]` call site in `handleLogin` to use the new `GetUser` method. Relocate exactly two test functions (`TestSessionExpiry`, `TestCleanupExpiredSessions`) verbatim into `internal/usecase/auth`; everything else in `auth_test.go`/`auth_coverage_test.go` stays in `internal/adapter/web` unmodified.
  - **Acceptance criteria:**
    - [ ] `internal/usecase/auth` package exists with `Service`, `AuthUser`, `Session`, and all moved/exported methods listed in `data-dictionary.md`.
    - [ ] `internal/adapter/web.AuthService` is a wrapper struct (not a type alias) embedding `*auth.Service`; `NewAuthService()` remains zero-arg.
    - [ ] `internal/adapter/web` compiles against the new package with no behavior change.
    - [ ] All pre-existing tests in `auth_test.go`/`auth_coverage_test.go` pass unmodified, except `TestSessionExpiry` and `TestCleanupExpiredSessions`, which are relocated with identical bodies into `internal/usecase/auth`.
    - [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds; `./bin/nuimanbot --help` runs.

- **P1.2 — Add `RestoreSession` with defense-in-depth hardening (AD-2)**
  - **Dependencies:** P1.1
  - **Description:** Add `auth.Service.RestoreSession(session *Session) error` per `architecture.md` AD-2: independently re-validate `ExpiresAt` against `time.Now()`, and confirm `session.Username` still exists via `GetUser` — reject (return error) on either failure. Do not add any signature/HMAC verification (explicitly out of scope per AD-2's decided threat model).
  - **Acceptance criteria:**
    - [ ] `RestoreSession` rejects an expired session even if the caller's own pre-check was wrong/bypassed.
    - [ ] `RestoreSession` rejects a session whose `Username` no longer exists in the live `users` map.
    - [ ] `RestoreSession` accepts a valid, non-expired session for an existing user and re-hydrates it into the in-memory `sessions` map such that a subsequent `ValidateSession`/`GetSession` call succeeds.
    - [ ] Unit tests cover all three cases above.

## Phase B: CLI Authentication (FR-001–FR-007, FR-044's CLI half)

- **P2.1 — Login prompt and fresh-login flow**
  - **Dependencies:** P1.1, P1.2
  - **Description:** New `internal/adapter/gateway/cli/auth_commands.go`. REPL start checks for a session file; absent → prompt for username/password (masked via `golang.org/x/term.ReadPassword` — add as a new `go.mod` dependency, confirmed absent from `go.sum` during Step 2 review) → `auth.Service.ValidateCredentials` + `CreateSession` → write session record to disk (`0600`, schema per `data-dictionary.md`) → proceed authenticated.
  - **Acceptance criteria:**
    - [ ] No valid session on disk → username/password prompted before any command or chat input is accepted (FR-001).
    - [ ] Password entry is masked in the terminal.
    - [ ] Successful login persists the session file with `0600` permissions.
    - [ ] Failed login does not leak whether the username exists vs. the password is wrong (reuses `auth.Service.ValidateCredentials`'s existing behavior — no new differential-timing/error-message path introduced).

- **P2.2 — Session restore on process restart, with fallback (FR-003/FR-004)**
  - **Dependencies:** P2.1
  - **Description:** On REPL start, read the session file if present; if `ExpiresAt` is future, call `RestoreSession`; on success, proceed authenticated with no prompt. On any failure (absent, corrupted/unparseable JSON, expired, or `RestoreSession` rejection), fall back to the P2.1 login prompt — never crash (Reliability NFR).
  - **Acceptance criteria:**
    - [ ] Restarting within the expiry window skips the login prompt.
    - [ ] A corrupted session file falls back to login without a panic/crash.
    - [ ] An expired session file falls back to login.

- **P2.3 — Identity reconciliation with `domain.User`/`userService` (AD-6)**
  - **Dependencies:** P2.1, P2.2
  - **Description:** Immediately after a successful fresh login or session restore, and before the REPL accepts any chat input, look up `userService.GetUserByPlatformUID(ctx, domain.PlatformCLI, session.Username)`. If found with a stale `Role`, update it to match the just-authenticated session's role (converted via `domain.Role(session.Role)`). If not found, create it directly with the correct role — do not let `chat.Service`'s `resolveUser`/`defaultRoleForPlatform` auto-create path run for an authenticated CLI user.
  - **Acceptance criteria:**
    - [ ] A non-admin (`RoleUser`) CLI login, followed by a plain-text chat message, does not result in a `domain.User` record with `RoleAdmin` for that username.
    - [ ] A previously-admin user who is demoted (role changed via web admin) between CLI sessions has their `domain.User` record's `Role` corrected on next CLI login/restore, not left stale.
    - [ ] Regression test: an admin user's existing tool-execution privileges during CLI chat are unaffected (no accidental downgrade).

- **P2.4 — `/logout` command (FR-005)**
  - **Dependencies:** P2.1
  - **Description:** `/logout` calls `auth.Service.DestroySession` and deletes the local session file.
  - **Acceptance criteria:**
    - [ ] After `/logout`, the next command requires login again.
    - [ ] The session file no longer exists on disk after `/logout`.

- **P2.5 — Replace auto-admin grant and hardcoded chat attribution (main.go)**
  - **Dependencies:** P2.1–P2.4
  - **Description:** Remove `cmd/nuimanbot/main.go`'s unconditional `cliGateway.SetCurrentUser(&domain.User{ID: "cli_admin", ...})` call; wire the login/restore flow to call `SetCurrentUser` with the real authenticated identity instead. Replace `skillHandler.SetMessageHandler(messageHandler, domain.PlatformCLI, "cli_user")`'s hardcoded `"cli_user"` with `session.Username`, consistent with AD-5's `ownerUserID` convention.
  - **Acceptance criteria:**
    - [ ] `SetCurrentUser` is only called with a real, logged-in `domain.User`, never the hardcoded `cli_admin`.
    - [ ] A chat message sent via the REPL is attributed to the logged-in user's real `Username`, verifiable in that Chat's stored history (both the old live-chat conversation path and, once built, the new Chats-environment path use the same identity string).

- **P2.6 — Role-gate existing admin commands (FR-006)**
  - **Dependencies:** P2.1–P2.5
  - **Description:** Gate skill/profile/bot/config/memory admin commands and Settings' system-wide half by the logged-in user's real `Role` in `gateway.go`'s dispatch, replacing today's implicit always-admin assumption.
  - **Acceptance criteria:**
    - [ ] A non-admin user's attempt to run an admin-only command is rejected with a clear permission error, not silently allowed.
    - [ ] All existing admin commands still work end-to-end for a logged-in admin user.

## Phase C: Environment Command Handlers (FR-008–FR-038, minus deferred chat sub-commands)

- **P3.1 — Chats (FR-011–FR-016)**
  - **Dependencies:** Phase B complete
  - **Description:** `chat_commands.go` — `list`/`show`/`new`/`send`/`delete`/`export`, calling `internal/usecase/chats.Service` with `ownerUserID = session.Username` (AD-5).
  - **Acceptance criteria:** all six subcommands work; a Chat created via CLI is visible via `GET /admin/chats` in the web UI and vice versa; two different users don't see each other's Chats.

- **P3.2 — Projects (FR-017–FR-022; FR-020 deferred, see spec.md)**
  - **Dependencies:** Phase B complete
  - **Description:** `project_commands.go` — `list`/`create`/`show`/`add-agents-file`/`delete`. No `chat` subcommand this pass (spec.md FR-020, deferred).
  - **Acceptance criteria:** all five implemented subcommands round-trip with the web UI; cross-user isolation verified.

- **P3.3 — Jobs (FR-023–FR-025, FR-027; FR-026 deferred)**
  - **Dependencies:** Phase B complete
  - **Description:** `job_commands.go` — `list [--project|--chat]`/`create [--project|--chat]`/`show`/`delete`. No `chat` subcommand this pass (spec.md FR-026, deferred).
  - **Acceptance criteria:** all four implemented subcommands round-trip with the web UI; `/job create --project <id>` where `<id>` belongs to another user returns a not-found/permission error (see spec.md's acceptance-criteria edge case), not silent cross-user attachment — explicit test required, not just reliance on `ownerUserID` scoping being correct by construction.

- **P3.4 — Chores (FR-028–FR-030, FR-032–FR-033; FR-031 deferred)**
  - **Dependencies:** Phase B complete
  - **Description:** `chore_commands.go` — `list`/`create --schedule`/`show`/`delete`/`confirm-schedule`. No `chat` subcommand this pass (spec.md FR-031, deferred).
  - **Acceptance criteria:** all five implemented subcommands round-trip with the web UI; soft-delete semantics match the web UI's Chore/Job delete parity fix.

- **P3.5 — History (FR-034–FR-035; FR-036 deferred)**
  - **Dependencies:** Phase B complete
  - **Description:** `history_commands.go` — `list [--job|--chore] [--status] [--since]`/`show`. No `chat` subcommand this pass (spec.md FR-036, deferred).
  - **Acceptance criteria:** both implemented subcommands round-trip with the web UI; large result sets paginate/truncate (Performance NFR).

- **P3.6 — Memories (FR-037–FR-038)**
  - **Dependencies:** Phase B complete
  - **Description:** `memories_commands.go` — `browse [query]` (read-only), `chat <cell-id> <message>` (calls `MemoriesService.AskAboutCell`, the one per-item chat capability that does have a real backing method). Dispatch in `gateway.go` uses `/memories` (plural) as a distinct, non-overlapping prefix from the existing `/memory` (singular) admin prefix — verified non-colliding per `architecture.md`'s Integration Points section, but still needs its own dispatch test, including bare `/memories` with no args showing help.
  - **Acceptance criteria:** `/memories browse` and `/memory stats` both dispatch correctly side by side (explicit test); `/memories chat` round-trips through `AskAboutCell`; bare `/memories` shows help, not an unrecognized-command error.

## Phase D: Settings (FR-039–FR-043)

- **P4.1 — Per-user Settings**
  - **Dependencies:** Phase B complete
  - **Description:** `settings_commands.go` — `show` and `set retention <chat|project|history> <value|never>`, per-user scope.
  - **Acceptance criteria:** both subcommands work for a non-admin user; retention values including `"never"` display/parse correctly.

- **P4.2 — Admin-only Settings**
  - **Dependencies:** P4.1, P2.6 (role-gating)
  - **Description:** `show --system`, `set worker-pool-size <n>`, `set network-mode <localhost|remote>`. The network-mode command's help text must explicitly state the known limitation (doesn't rebind the running listener), matching the web UI's own admission — not overpromise.
  - **Acceptance criteria:** all three subcommands are admin-gated (rejected for non-admin); `set network-mode`'s help text states the limitation.

## Phase E: Cleanup and Docs

- **P5.1 — Deprecation note for auto-admin removal**
  - **Dependencies:** Phase B complete
  - **Description:** Add a `support_docs/` note documenting that `./bin/nuimanbot` no longer grants immediate admin access — a breaking change for any local scripts/ops runbooks relying on today's zero-friction CLI admin access, per spec.md's Breaking Changes section.
  - **Acceptance criteria:** note exists in `support_docs/`, referenced from release notes per `AGENTS.md`'s documentation-maintenance rules.

- **P5.2 — Update `documentation/` product docs**
  - **Dependencies:** All prior phases complete
  - **Description:** Update `documentation/product-summary.md`, `product-details.md`, `technical-details.md` and root `README.md` per `AGENTS.md`'s mandatory documentation-maintenance rule for architecturally significant changes (this feature: new `internal/usecase/auth` package, CLI login model change, seven new command families).
  - **Acceptance criteria:** all four files updated in the same change as the code; no stale references to the old auto-admin CLI behavior remain.

- **P5.3 — Final quality-gate pass**
  - **Dependencies:** All prior phases complete
  - **Description:** Run the full `AGENTS.md` quality-gate chain: `go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help`.
  - **Acceptance criteria:** all seven steps pass with zero errors; `internal/adapter/gateway/cli` does not import `internal/adapter/web` (verify via `go list -deps` or equivalent).
