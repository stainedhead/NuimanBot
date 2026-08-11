# Architecture: CLI Parity for NuimanBot Features

**Feature:** cli-parity-for-nuimanbot-features
**Created:** 2026-08-11
**Status:** Reviewed and finalized during pipeline Step 2 (Review Spec, `dev-flow:review-spec`). AD-1 and AD-2 were re-examined against the actual current code (`internal/adapter/web/auth.go`, its test files, `internal/usecase/chat/service.go`, and all six existing web `*Service` handler interfaces) and materially revised — the original drafts (type-alias for AD-1, an open threat-model question for AD-2) did not survive contact with the code. See "Review Findings (Step 2)" below for what changed and why. AD-5 and AD-6 are new decisions added during this review.

## Architecture Overview

This feature touches three layers:

1. **`internal/usecase/auth` (new)** — houses the credential-verification and session-management logic extracted from `internal/adapter/web/auth.go`'s `AuthService`, per FR-044. Consumed by both `internal/adapter/web` and the new CLI auth flow.
2. **`internal/adapter/web` (modified)** — keeps HTTP-transport concerns (cookies, CSRF, middleware, HTTP handlers); delegates credential/session logic to `internal/usecase/auth`. Behavior/UX unchanged.
3. **`internal/adapter/gateway/cli` (modified/new)** — gains a login/logout REPL flow backed by `internal/usecase/auth`, plus new environment command handlers (Chats/Projects/Jobs/Chores/History/Memories/Settings) backed by the existing `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` services.

The riskiest piece — and the one this document prioritizes — is the `AuthService` extraction (FR-044), because it is a genuine refactor of code the web UI depends on today, done without changing web behavior, while also making the extracted service usable by a fundamentally different runtime shape (a long-lived server process for web vs. a REPL that can be stopped and restarted for CLI).

## Architectural Decision AD-1: AuthService Extraction Split

**Decision:** Split `internal/adapter/web/auth.go` along the "credential/session logic" vs. "HTTP transport" line, not a blanket move of the whole file.

**Moves to `internal/usecase/auth` (new package):**

| Current symbol (`web` package) | New home |
|---|---|
| `AuthService` struct (rename to `auth.Service`) | `internal/usecase/auth` |
| `AuthUser` struct | `internal/usecase/auth` |
| `Session` struct | `internal/usecase/auth` |
| `NewAuthService()` → `auth.NewService(...)` | `internal/usecase/auth` |
| `AddUser`, `ValidateCredentials`, `UpdatePassword`, `isDefaultCredentials` | `internal/usecase/auth` |
| `CreateSession`, `createSessionWithFlags`, `ValidateSession`, `GetSession`, `DestroySession`, `ClearForcePasswordChange` | `internal/usecase/auth` |
| `runCleanupLoop`, `cleanupExpiredSessions` | `internal/usecase/auth` |
| `generateRandomString` | `internal/usecase/auth` (unexported helper) |

**Stays in `internal/adapter/web` (HTTP-transport concerns, not reusable by CLI):**

| Symbol | Reason |
|---|---|
| `GenerateCSRFToken`, `ValidateCSRFToken`, `csrfTokens` map | CSRF protects browser form submissions against cross-site forgery; meaningless for a local terminal REPL with no browser/cookie surface. Kept as a thin wrapper in `web` that composes with the extracted service, or folded into a small `web`-local CSRF helper independent of `auth.Service`. |
| `sessionCookieName`, `secureCookies` field, `setSecureCookies`/`isSecureCookies` | Cookie transport is HTTP-specific. |
| `Server.SetAuthService`, `Server.RequireAuth` (middleware), `handleLogin`, `handleLogout`, `handleChangePassword`, `getCurrentUser` | HTTP handler/middleware layer; rewritten to call `auth.Service` methods instead of local `AuthService` methods — signatures preserved so behavior/UX (session length, login form, password rules) is unchanged. |

**Compatibility approach — REVISED during Step 2 review, do not use a bare type alias for `AuthService` itself:**

A first draft of this decision proposed `type AuthService = auth.Service` (a type alias) in `web`. **Verified against the actual code and rejected** — a type alias for a struct defined in another package does not, and cannot, expose that package's unexported fields/methods. Concretely, if `AuthService` becomes an alias for `auth.Service`:

- `internal/adapter/web/auth.go:332` (`handleLogin`): `user, exists := s.auth.users[username]` — direct access to the unexported `users` map — **fails to compile**, `users` is no longer in package `web`.
- `internal/adapter/web/tls.go:21`: `s.auth.setSecureCookies(true)` — unexported method call — **fails to compile**.
- `internal/adapter/web/auth.go:339,342`: `s.auth.isDefaultCredentials(...)`, `s.auth.createSessionWithFlags(...)` — unexported method calls — **fail to compile**.
- `internal/adapter/web/auth_test.go:196`: `auth.sessionTimeout = 100 * time.Millisecond` (`TestSessionExpiry`) — unexported field write — **fails to compile**.
- `internal/adapter/web/auth_coverage_test.go` — `TestSetSecureCookies` (calls `setSecureCookies`/`isSecureCookies`), `TestCreateSessionWithFlags` and several other tests (call `createSessionWithFlags`), `TestIsDefaultCredentials` (calls `isDefaultCredentials`), `TestCleanupExpiredSessions` (reads `auth.sessions[sessionID]` directly) — **all fail to compile**.

That last group is the acceptance-criterion-breaking one: a type alias makes it *impossible*, not just inconvenient, to satisfy "the existing web auth tests still pass unmodified" for `auth_coverage_test.go`, because Go's unexported-identifier visibility is scoped to the *declaring* package, and aliasing doesn't change which package declared the identifier.

**Revised decision: `web.AuthService` is a wrapper struct that embeds `*auth.Service`, not an alias.**

```go
// internal/adapter/web/auth.go (post-refactor)
type AuthService struct {
    *auth.Service                    // promotes ValidateCredentials, CreateSession,
                                      // ValidateSession, GetSession, DestroySession,
                                      // UpdatePassword, ClearForcePasswordChange,
                                      // CreateSessionWithFlags, IsDefaultCredentials,
                                      // GetUser, RestoreSession — all exported on auth.Service
    csrfTokens    map[string]bool    // stays in web per this doc's original AD-1 table — unchanged
    secureCookies bool               // stays in web — unchanged
    mu            sync.RWMutex       // guards csrfTokens/secureCookies only; auth.Service has its own internal locking
}

func NewAuthService() *AuthService {   // stays a zero-arg constructor — ~8 existing test call sites do `auth := NewAuthService()`
    return &AuthService{
        Service:    auth.NewService(),
        csrfTokens: make(map[string]bool),
    }
}

// Unexported shims, declared in package web, so that existing white-box tests
// in package web (auth_coverage_test.go) that call the lowercase names keep compiling unmodified:
func (a *AuthService) setSecureCookies(secure bool) { a.mu.Lock(); defer a.mu.Unlock(); a.secureCookies = secure }
func (a *AuthService) isSecureCookies() bool         { a.mu.RLock(); defer a.mu.RUnlock(); return a.secureCookies }
func (a *AuthService) createSessionWithFlags(username, role string, forceChange bool) string {
    return a.Service.CreateSessionWithFlags(username, role, forceChange)
}
func (a *AuthService) isDefaultCredentials(username, password string) bool {
    return a.Service.IsDefaultCredentials(username, password)
}
// GenerateCSRFToken / ValidateCSRFToken stay declared directly on AuthService (web), operating on
// the wrapper's own csrfTokens field — not delegated to auth.Service at all, per this doc's
// original classification (CSRF is HTTP-transport-only, meaningless for the CLI).
```

Why this works where the alias didn't: `setSecureCookies`, `isSecureCookies`, `createSessionWithFlags`, `isDefaultCredentials` are *declared in package `web`* (as thin shims), so `auth_coverage_test.go` (also package `web`) calling `auth.setSecureCookies(true)` / `auth.createSessionWithFlags(...)` / `auth.isDefaultCredentials(...)` / `auth.isSecureCookies()` continues to compile and pass **unmodified** — same source, same package, same lowercase names.

**Two test functions are the one place "unmodified" is not literally achievable, and that's a documented, narrow exception, not a redefinition of the criterion:**
- `TestSessionExpiry` (`auth_test.go:195`) writes the unexported `sessionTimeout` *field* directly (`auth.sessionTimeout = ...`) — a struct-embedding shim can't intercept a field write the way it can a method call. This field now lives inside the embedded `auth.Service`, in a different package.
- `TestCleanupExpiredSessions` (`auth_coverage_test.go:376`) reads `auth.sessions[sessionID]` directly — same problem, unexported map field.

**Resolution:** these two tests move verbatim (identical test body/assertions, only the package clause and import path change) into `internal/usecase/auth/session_test.go`, since they are, in substance, tests of session-management logic that itself moved there. This is a two-function, explicitly-documented exception — everything else in `auth_test.go`/`auth_coverage_test.go` (roughly 30 of 32 test functions) stays in `internal/adapter/web`, unmodified, and must pass against the refactored wrapper.

**Production-code call sites that must change (allowed — these are not test files, and AD-1 already anticipated "a handful of call sites" needing updates):**
- `auth.go:332` — `s.auth.users[username]` → replace with a new exported `auth.Service` method: `func (s *Service) GetUser(username string) (AuthUser, bool)`. `handleLogin` becomes `user, exists := s.auth.GetUser(username)`.
- `tls.go:21` — `s.auth.setSecureCookies(true)` — unaffected; `setSecureCookies` stays a web-package method per the shim above, so this call site needs no change at all.
- `auth.go:339,342` — `s.auth.isDefaultCredentials(...)`, `s.auth.createSessionWithFlags(...)` — unaffected for the same reason (shims preserve the exact call syntax).
- `server.go:308` — `GetAuth() *AuthService` — return type is now the wrapper struct, not `*auth.Service`. `server_coverage_test.go:22`'s `TestGetAuth` only nil-checks the result, so it's unaffected. `cmd/nuimanbot/main.go`'s CLI-auth wiring must use the *embedded* `*auth.Service` (via `webServer.GetAuth().Service`) when constructing the CLI's login/session flow, not the wrapper — the CLI adapter has no legitimate use for `csrfTokens`/`secureCookies`.

**Verification:** run `go build ./... && go test ./internal/adapter/web/... ./internal/usecase/auth/...` after the extraction. All pre-existing `internal/adapter/web` test names must still exist, in the same file, with the same assertions, except the two relocations documented above.

## Architectural Decision AD-2: Session Persistence Across CLI Process Restarts

**Problem identified during architecture review:** the current `AuthService.sessions` is an in-memory `map[string]*Session`, appropriate for a web server that is one long-running process serving many browser sessions. The CLI has a different lifecycle: FR-003/FR-004 require that a session token persisted to local disk let a **new** process invocation (e.g. the user closes the terminal and reopens it) skip re-login until the token expires. If `internal/usecase/auth.Service` keeps sessions only in an in-memory map, a fresh CLI process starts with an empty map — a disk-persisted session **ID** alone is not enough to satisfy `ValidateSession`/`GetSession`, because there would be nothing in memory to validate against.

**Decision:** the CLI adapter persists the **full session record** (not just the ID) to disk — `{session_id, username, role, created_at, expires_at}` as JSON, file mode `0600`, located alongside the existing `HistoryFile` path convention in `internal/adapter/gateway/cli`. On REPL start, the CLI adapter:

1. Reads the local session file, if present.
2. If present and `expires_at` is in the future, calls a new `auth.Service` method — `RestoreSession(session *auth.Session) error` (or equivalent) — to re-hydrate that session into the service's in-memory store, then proceeds as authenticated. No re-login prompt (FR-003).
3. If absent, corrupted, or expired, falls back to the login prompt (FR-004, and the Reliability NFR: "a missing or corrupted session-token file falls back to a fresh login prompt rather than crashing the REPL").
4. On successful fresh login, writes the newly created `auth.Session` to the local file.
5. On `/logout` (FR-005), deletes the local file and calls `DestroySession` on the in-memory store.

This keeps `internal/usecase/auth.Service`'s public surface additive (one new `RestoreSession` method) rather than requiring a pluggable `SessionStore` interface — the simpler of two designs considered (see Alternatives). The disk file itself is entirely a CLI-adapter concern; `internal/usecase/auth` remains storage-agnostic and unaware that the CLI persists sessions across restarts at all.

**Alternatives considered:**
- *Pluggable `SessionStore` interface* inside `internal/usecase/auth` (in-memory default for web, disk-backed implementation injected by CLI). Rejected for this pass as higher-surface-area than needed — `RestoreSession` achieves the same outcome with a single new method and keeps the web adapter's behavior/wiring completely untouched, honoring the PRD's Non-Goal of not redesigning web auth.
- *CLI re-validates independently of the shared service* (i.e., CLI checks `expires_at` itself and never calls into `auth.Service` at all for restored sessions, only for fresh logins). Rejected — this would create a second, weaker path to "being authenticated" that bypasses the shared service, directly contradicting the PRD's stated principle (echoed in FR-044's acceptance criterion) that both adapters must depend on the one shared service, not reimplement logic independently.

**Session-token expiry duration (resolved, was `research.md` Q2):** default to `24 * time.Hour`, matching the web admin's existing `sessionTimeout` constant in `internal/adapter/web/auth.go`. No product reason surfaced during this review to diverge; using the same default keeps CLI and web session lifetimes easy to reason about together, and `auth.Service.sessionTimeout` is already a single shared field once the extraction lands — the CLI does not need (and this pass does not add) a separate configurable duration knob. Revisit only if usage data later shows CLI sessions expiring mid-task too often.

**`RestoreSession` hardening (resolved — was an open follow-up, now a firm decision, split into what it fixes and what it deliberately does not):**

1. **Independent expiry re-check (defense in depth).** `RestoreSession(session *Session) error` must re-check `session.ExpiresAt` against `time.Now()` itself and reject (return an error, causing the CLI to fall back to a fresh login prompt) rather than trusting the CLI adapter's own pre-check. Two independent checks catch a CLI-adapter bug (e.g. a clock-skew or off-by-one error in the file-reading code) that a single check would not.
2. **Fail closed if the username no longer exists.** `RestoreSession` must look up `session.Username` in `auth.Service`'s own `users` map (via the new `GetUser` method from AD-1) and reject the restore if the account was deleted since the session was persisted — same reasoning as expiry: don't trust stale disk state where the live source of truth disagrees.
3. **Re-deriving `Role` from the live user store is a correctness fix, not a security fix — do not present it as closing a tampering hole, because it doesn't.** A first draft of this decision suggested "re-derive `Role` from the user store rather than trusting the persisted file's `role` field, to avoid a stale-privilege window" as if this closed a local-tampering attack (a demoted admin re-granting themselves admin by hand-editing the session file). **It does not close that attack**, because the same attacker can edit the persisted `username` field instead of `role` — if they overwrite it with `"admin"`, `RestoreSession` looks up `"admin"` in the live user store, finds it, re-derives `Role = "admin"` correctly per *that* username, and grants it. The forgery just moves from one JSON field to another. Re-deriving `Role` is still worth doing (it fixes the legitimate case of an admin being demoted through the web UI between CLI sessions — the session file goes stale in a *good-faith* way, not an adversarial one), but it must not be described as a defense against a malicious local user.
4. **Decided threat model (this is the actual resolution, not a deferred question):** the CLI's persisted session file is protected by OS file permissions (`0600`) and its trust boundary is the OS user account, not the CLI process. A user with write access to their own session file already has everything that access implies: they can run `./bin/nuimanbot` themselves and log in as any account whose password they know, or — if they've compromised the OS account enough to edit arbitrary files owned by it — they already have full access to whatever that OS account can reach, session file or not. Editing your own session file to claim a different `username` you are not the legitimate owner of is credential forgery, exactly as picking a username/password pair to guess at the login prompt is — it is bounded by whether `auth.Service`'s `users` map has that account and the attacker can produce a session record `RestoreSession` accepts, not by anything `RestoreSession`'s validation logic can add. This is explicitly consistent with the PRD's Non-Goal ruling out networked/multi-user CLI access: the design assumes one OS user account maps to at most one trusted local operator. Multi-tenant sharing of a single OS account's CLI installation across untrusted parties is out of scope; if that ever becomes a requirement, it needs a signed/MACed session token (e.g. HMAC over the session fields with a server-held key), which is deliberately not built here.
5. **What `RestoreSession` is NOT responsible for:** validating that the persisted `session_id` was actually issued by `CreateSession` (i.e., no signature/HMAC check). Given point 4's threat model, this is consistent — the boundary is the OS file permission, not the token's unforgeability, since the same local user who could forge a `session_id` could equally well just log in normally.

## Review Findings (Step 2) — New Decisions AD-5, AD-6

Two more load-bearing facts surfaced only by reading the actual current code (not just the PRD's description of it). Both change the shape of implementation and both are resolved below, not left open.

### AD-5: `ownerUserID` across all six environment services is the session's `Username`, not `User.ID`

Verified by grep across all six existing web handlers (`chats_handler.go`, `projects_handler.go`, `jobs_handler.go`, `chores_handler.go`, `history_handler.go`, `memories_handler.go`): every single call into `internal/usecase/{chats,projects,jobs,chores,history,memories}` passes `user.Username` as `ownerUserID`, never `user.ID`. This is explicit and intentional — `projects_handler.go`'s `ProjectsService` doc comment states it directly: *"ownerUserID is the current session's Username (see ChatsService's doc comment for why — session.ID is a per-session token, not a stable user identifier)."*

**Consequence for this feature:** the CLI adapter must use the logged-in `Session.Username` (not `Session.ID`, not any `domain.User.ID`) as the `ownerUserID` argument to every one of the six new environment handlers. Getting this wrong doesn't fail loudly — it silently creates a parallel, invisible set of CLI-only records that never show up in the web UI and vice versa, directly breaking the acceptance criterion "data created via CLI is visible in the web UI and vice versa." Added to `data-dictionary.md`.

### AD-6: `internal/usecase/chat/service.go`'s `defaultRoleForPlatform` is a second, independent "CLI = trusted = admin" shortcut this PRD must also close

`resolveUser` (the RBAC entry point used by the CLI's existing live-chat pipeline — the plain-text, non-slash-command chat path that FR-007 targets) auto-creates a `domain.User` on first message from a never-before-seen `(platform, platformUID)` pair, via `defaultRoleForPlatform(platform)`:

```go
// defaultRoleForPlatform ... CLI is deliberately special-cased to RoleAdmin
// rather than RoleGuest: it's inherently local/trusted ... This preserves
// the CLI's pre-existing de facto unrestricted access.
func defaultRoleForPlatform(platform domain.Platform) domain.Role {
    if platform == domain.PlatformCLI {
        return domain.RoleAdmin
        ...
```

This is a **second, independent place** in the codebase that encodes exactly the assumption FR-006/FR-007 exist to remove ("CLI = implicit trusted admin"), and it is not touched by anything else in this architecture document. If FR-007 is implemented as simply "replace the hardcoded `\"cli_user\"` literal with `session.Username`" (the natural fix, and the one consistent with AD-5's convention), the consequence is: **every distinct logged-in CLI username, on their first plain-chat message, gets a freshly auto-provisioned `domain.User` with `RoleAdmin`** via this path — regardless of the `Role` `auth.Service` actually authenticated them with. A non-admin user logging in via the new real-auth flow (FR-001/002/006) would still get de facto RBAC admin tool-execution privileges the moment they send their first chat message, because `chat.Service.ProcessMessage`'s RBAC resolution never consults `auth.Service` at all — it has its own, entirely separate `domain.User`/`UserRepository` identity model, keyed by `PlatformIDs`, that predates this feature.

**Decision:** the CLI's post-login flow (immediately after a successful login or session restore, before the REPL accepts any chat input) must reconcile the two identity systems: call `userService.GetUserByPlatformUID(ctx, domain.PlatformCLI, session.Username)`; if found, and its `Role` doesn't match the just-authenticated `auth.Service` session's `Role`, update it (via whatever `UserRepository`/`userService` update path already exists — reuse, don't add a second one) to match; if not found, create it directly with the session's real `Role` rather than letting `resolveUser`'s auto-create path run with `defaultRoleForPlatform`'s hardcoded default. This guarantees `resolveUser`'s lookup always hits an existing, correctly-privileged record and its auto-create/default-role branch is never exercised for an authenticated CLI user. `defaultRoleForPlatform`'s `PlatformCLI` case itself is left unchanged (out of scope to touch shared RBAC code for other platforms) — the CLI login flow simply ensures its branch is never reached post-login. Added as a required task in `tasks.md` Phase B; this is a compile-clean, easy-to-miss correctness gap, not a nice-to-have.

### Scope correction: three of the five "per-item agent chat" sub-commands have no backing web implementation — descoped, not built new

Verified against the actual merged web code (post-PR #8): the `ProjectsService`, `JobsService`, `ChoresService`, and `HistoryService` interfaces in `internal/adapter/web/{projects,jobs,chores,history}_handler.go` expose only list/get/create/delete(/`ConfirmSchedule` for Chores) — **none of them has a chat/converse method**. This is explicit and intentional in the existing code, not an oversight this review is catching early:

- `internal/adapter/web/jobs_handler.go:155` — comment: *"own chat interface (FR-029) is out of scope for this pass"*
- `internal/adapter/web/chores_handler.go:102` — comment: *"chat interface, not this form"* (i.e., exists conceptually, not implemented)
- `internal/adapter/web/history_handler.go:230` — comment: *"per-run chat interface is out of scope for this environment"*
- Projects has no comment, but likewise no chat method on `ProjectsService`.

Only Memories has a real backing method: `MemoriesService.AskAboutCell(ctx, ownerUserID, cellID, question) (string, error)`.

**This directly contradicts the PRD's own Non-Goal** ("no new capabilities beyond what the web UI already has") if FR-020 (`/project chat`), FR-026 (`/job chat`), FR-031 (`/chore chat`), and the chat portion of FR-036 (`/history chat`) were implemented as literally specified — there is nothing server-side to mirror; building them would mean inventing new usecase-layer chat capability, which is explicitly out of scope for this pass.

**Decision:** FR-020, FR-026, FR-031, and FR-036's chat sub-command are **deferred**, matching the web UI's own already-documented deferral. This spec builds only the CLI commands that have a real backing service method today: `/project list|show|create|delete|add-agents-file`, `/job list|show|create|delete`, `/chore list|show|create|delete|confirm-schedule`, `/history list|show`. `/memories chat` stays in scope (backed by `AskAboutCell`). See `spec.md` for the corresponding FR text changes and updated acceptance criteria.

## Component Architecture

```
internal/usecase/auth/            (new)
  service.go         — Service struct, NewService, AddUser, ValidateCredentials, GetUser (new, AD-1),
                        UpdatePassword, CreateSessionWithFlags (exported, AD-1), IsDefaultCredentials
                        (exported, AD-1)
  session.go         — Session, AuthUser, CreateSession, ValidateSession, GetSession, DestroySession,
                        ClearForcePasswordChange, RestoreSession (new, AD-2), cleanup loop
  service_test.go, session_test.go — unit tests; almost entirely NEW tests for the new package's own
                        surface (GetUser, RestoreSession, exported CreateSessionWithFlags/
                        IsDefaultCredentials), plus exactly two tests relocated verbatim from
                        internal/adapter/web per AD-1's documented exception: TestSessionExpiry,
                        TestCleanupExpiredSessions.

internal/adapter/web/
  auth.go            — AuthService wrapper struct (embeds *auth.Service; owns csrfTokens,
                        secureCookies), NewAuthService (zero-arg, unchanged signature), unexported
                        shims (setSecureCookies/isSecureCookies/createSessionWithFlags/
                        isDefaultCredentials) so existing white-box tests keep compiling unmodified,
                        GenerateCSRFToken/ValidateCSRFToken (unchanged, web-only),
                        Server.RequireAuth, handleLogin/handleLogout/handleChangePassword (unchanged
                        signatures, call sites into auth.Service's exported methods where the old
                        AuthService had unexported ones — see AD-1's "production call sites" list).

internal/adapter/gateway/cli/
  auth_commands.go   — (new) login prompt (masked password entry via golang.org/x/term — new
                        dependency, not currently in go.mod), /logout command, session file read/
                        write (0600), RestoreSession call, and the AD-6 identity-reconciliation step
                        (sync domain.User's Role for (PlatformCLI, session.Username) before the REPL
                        accepts chat input).
  chat_commands.go, project_commands.go, job_commands.go, chore_commands.go,
  history_commands.go, memories_commands.go, settings_commands.go   — (new) one handler each,
                        following the existing cli.New*Handler + Gateway.Set*Handler pattern. Per the
                        Scope Correction above: project/job/chore/history handlers expose
                        list/show/create/delete(/confirm-schedule) only, no chat sub-command;
                        memories_commands.go's chat sub-command calls AskAboutCell.
  gateway.go         — new Set*Handler methods; dispatch updated to route /chat, /project, /job,
                        /chore, /history, /memories, /settings alongside existing /memory, /skill, etc.
                        Session/role check gates command dispatch before invoking any admin handler.
                        ownerUserID passed to every new handler is session.Username (AD-5), never
                        session.ID or any domain.User.ID.

cmd/nuimanbot/main.go
  — auto-admin SetCurrentUser call replaced by: construct auth.Service (shared with the web Server,
    via webServer.GetAuth().Service — see AD-1), wire into cliGateway's new auth-commands handler,
    trigger login-or-restore flow before REPL loop starts. Hardcoded "cli_user"
    (skillHandler.SetMessageHandler's third arg) replaced with session.Username, consistent with AD-5.
```

## Layer Responsibilities

| Layer | Responsibility |
|---|---|
| `internal/usecase/auth` | Credential verification, in-memory session lifecycle, session restoration hook (with its own independent expiry + user-existence re-check per AD-2, not just a re-hydration). No knowledge of HTTP or terminal I/O. |
| `internal/adapter/web` | HTTP transport: cookies, CSRF, request/response handling. Delegates all credential/session logic to the embedded `internal/usecase/auth.Service` (AD-1's wrapper struct, not a type alias). |
| `internal/adapter/gateway/cli` | Terminal I/O: login prompt, password masking (`golang.org/x/term`, new dependency), session-file persistence (disk read/write, permissions), post-login identity reconciliation with `domain.User`/`userService` (AD-6), command dispatch/gating, new environment command handlers delegating to `internal/usecase/{chats,projects,...}` using `session.Username` as `ownerUserID` (AD-5). |
| `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` | Unchanged — existing per-user-scoped business logic, already consumed by web, now also consumed by CLI. |
| `internal/usecase/chat` (existing, pre-dates this feature) | Unchanged code, but its `defaultRoleForPlatform`/`resolveUser` RBAC auto-provisioning path becomes reachable-but-must-never-fire for authenticated CLI users once AD-6's reconciliation step runs — see AD-6. |

## Data Flow

**Web login (unchanged behavior, new internal call path):**
`browser POST /login` → `web.handleLogin` → `auth.Service.ValidateCredentials` → `auth.Service.CreateSession` → `web` sets cookie → response.

**CLI fresh login (FR-001/FR-002/FR-003):**
REPL start → no valid session file → prompt username/password → `auth.Service.ValidateCredentials` → `auth.Service.CreateSession` → CLI adapter writes session record to disk (`0600`) → REPL proceeds authenticated.

**CLI restart within expiry (FR-003):**
REPL start → session file present, not expired (per CLI adapter's own check) → CLI adapter reads file → `auth.Service.RestoreSession(session)` — which independently re-checks `ExpiresAt` and looks up `session.Username` in the live `users` map, failing closed on either mismatch (AD-2 hardening) — → on success, CLI runs the AD-6 identity-reconciliation step against `domain.User`/`userService` → REPL proceeds authenticated, no prompt.

**CLI restart after expiry / missing / corrupted file / `RestoreSession` rejection (FR-004, Reliability NFR):**
REPL start → file absent, unparseable, `expires_at` in the past (either check), or `RestoreSession` returns an error (e.g. username no longer exists) → fall back to fresh-login prompt (never crash).

**CLI logout (FR-005):**
`/logout` → `auth.Service.DestroySession` → CLI adapter deletes local session file → subsequent commands require login again.

**CLI environment command, e.g. `/project list` (FR-017 et al.):**
Parsed by `gateway.go` dispatch → role/session check (any authenticated user, not admin-gated) → `project_commands.go` handler → `internal/usecase/projects.Service.ListProjects(ctx, session.Username)` (AD-5: `ownerUserID = session.Username`) → formatted terminal output.

## Sequence Diagrams

[Detailed sequence diagrams (login, restore, logout, admin-gated command rejection) to be added during the dedicated architecture-planning pass — flows are described textually above in Data Flow and are sufficient to unblock Step 2 (Review Spec) and early implementation planning.]

## Integration Points

- `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` — existing services, consumed as-is by new CLI handlers (no interface changes anticipated), always with `ownerUserID = session.Username` (AD-5).
- `internal/adapter/gateway/cli/memory_commands.go`'s `IsMemoryCommand()` — **verified, not just recommended:** `IsMemoryCommand` is `strings.HasPrefix(input, "/memory ")` (note the trailing space, already present in the existing code). This already does not collide with `/memories ...` — the 7th character of `"/memory "` is `'y'` while the 7th character of `"/memories"` is `'i'`, so the two prefixes diverge before the space matters. AD-3 (below) is therefore confirmed low-risk, not an open design question: the new `/memories` dispatch check just needs the same discipline (check the full token, e.g. `input == "/memories" || strings.HasPrefix(input, "/memories ")`, not a bare `strings.HasPrefix(input, "/memories")` that would itself be fine here but should still be written defensively). One gap: bare `/memories` with no trailing space/args (e.g. user just types `/memories` and hits enter) must still route to the new handler's help output (FR-010) rather than falling through unrecognized — add this as an explicit test case.
- `internal/usecase/chat` — `resolveUser`/`defaultRoleForPlatform`'s CLI-admin auto-provisioning path must be pre-empted by the CLI login flow's identity-reconciliation step (AD-6) before any chat message is processed.
- `cmd/nuimanbot/main.go` — single integration point where `auth.Service` is constructed and wired into both the web `Server` (existing call site, now via the `AuthService` wrapper) and the new CLI auth-commands handler (via the wrapper's embedded `*auth.Service`); the auto-admin `SetCurrentUser` call and the hardcoded `"cli_user"` `SetMessageHandler` argument are both replaced here, the latter with `session.Username`.

## Architectural Decisions (Summary)

- **AD-1 (revised during Step 2 review):** Split `AuthService` along credential/session (→ `internal/usecase/auth`) vs. HTTP-transport (stays in `internal/adapter/web`) lines. **`internal/adapter/web.AuthService` is a wrapper struct embedding `*auth.Service`, not a type alias** — a type alias was the original proposal and does not compile against the existing white-box tests (`auth_coverage_test.go`) or two production call sites (`auth.go:332`'s `users` map access, plus the `isDefaultCredentials`/`createSessionWithFlags` unexported-method calls); see the "Compatibility approach — REVISED" section above for the full analysis and the two-test-function exception this required.
- **AD-2 (revised during Step 2 review):** CLI persists full session records to disk (not just an ID) and re-hydrates them into the shared service via a new `RestoreSession` method on process restart, rather than introducing a pluggable session-store interface. `RestoreSession` independently re-validates expiry and username-existence (defense in depth). Its threat model is now explicit and decided (not an open follow-up): the trust boundary is the OS user account via `0600` file permissions; `RestoreSession` cannot and does not defend against a user forging their own session file to claim a different identity — that is accepted, out-of-scope risk consistent with the PRD's single-local-operator Non-Goal, not a residual gap.
- **AD-3 (verified during Step 2 review, no longer just "recommended"):** `/memories` vs `/memory` prefix dispatch — confirmed non-colliding today (`IsMemoryCommand`'s existing trailing-space form), new `/memories` dispatch must match the same discipline plus a bare-`/memories`-with-no-args test.
- **AD-4 (recommended, to confirm in planning):** New CLI environment handlers follow the existing `cli.New*Handler` + `Gateway.Set*Handler` pattern exactly (constructor takes the relevant `internal/usecase/*` service plus the current authenticated user/role, `Set*Handler` stores it on `Gateway`, dispatch routes by first token) — no new dispatch mechanism introduced.
- **AD-5 (new, Step 2 review):** `ownerUserID` for all six environment services is always `session.Username`, never `session.ID`/`domain.User.ID` — verified by grep across all six existing web handlers.
- **AD-6 (new, Step 2 review):** The CLI login flow must reconcile `domain.User`'s `Role` (via `userService`) for `(PlatformCLI, session.Username)` immediately after authentication, so `internal/usecase/chat`'s pre-existing `defaultRoleForPlatform(PlatformCLI) = RoleAdmin` auto-provisioning path is never reached for a real, authenticated CLI identity — otherwise every CLI user's first chat message silently grants them RBAC admin tool-execution privileges regardless of their actual `auth.Service` role, defeating FR-006.
