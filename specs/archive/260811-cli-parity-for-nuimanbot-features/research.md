# Research: CLI Parity for NuimanBot Features

**Feature:** cli-parity-for-nuimanbot-features
**Created:** 2026-08-11
**Source PRD:** [cli-parity-for-nuimanbot-features-PRD.md](./cli-parity-for-nuimanbot-features-PRD.md)

## Research Questions

Derived from the PRD's Open Questions and Dependencies/Risks sections:

1. **AuthService extraction shape (FR-044, highest risk):** What is the minimal-diff way to move `internal/adapter/web/auth.go`'s `AuthService` (`ValidateCredentials`, `CreateSession`, `ValidateSession`, `GetSession`, `DestroySession`, session cleanup loop, CSRF token handling, `UpdatePassword`, `isDefaultCredentials`, cookie-secure flag) into `internal/usecase/auth` while keeping the web adapter's HTTP-specific pieces (cookie handling, `RequireAuth` middleware, `handleLogin`/`handleLogout`/`handleChangePassword` HTTP handlers, CSRF) in `internal/adapter/web`? Which methods are pure credential/session logic (usecase-layer) vs. HTTP-transport concerns (adapter-layer, stay put)?
2. **Session-token expiry duration:** Not pinned by the PRD — what default should the CLI use? Should it match the web's existing `sessionTimeout = 24 * time.Hour`, or use a separate (possibly longer, given CLI's lower-frequency usage pattern) duration?
3. **CLI password entry UX:** Masked terminal input (e.g. via `golang.org/x/term.ReadPassword`) vs. plain-echo input — what's already available in the module's dependency graph (check `go.mod` for an existing terminal/password library), and what's the minimal addition?
4. **`/settings set network-mode` CLI exposure:** Should the CLI block this entirely (admin must use web UI) given it can't fully apply from either surface today (known pre-existing limitation, FR-R11), or expose it with the same caveat the web UI already carries? Needs revisiting once/if the underlying web Settings limitation is fixed.
5. **Session-token persistence format/location:** The PRD says "alongside `HistoryFile`, OS-permission-scoped" (e.g. `0600`) — what is the existing `HistoryFile` path/config mechanism in `internal/adapter/gateway/cli`, and can the session-token file reuse the same config/path-resolution logic?

## Industry Standards

Not applicable as a separate research track — resolved by following the existing in-repo convention instead of introducing a new one. The PRD's own FR-003 already specifies "alongside `HistoryFile`," and `data-dictionary.md` confirms `HistoryFile`'s path is resolved via `internal/config.CLIConfig.HistoryFile` (`internal/config/gateway_config.go`). The session-token file reuses that same config/path-resolution mechanism rather than adopting XDG Base Directory conventions net-new — consistent with this repo's existing pattern, and avoids a second, parallel path-resolution scheme for one small file.

## Existing Implementations

- `internal/adapter/web/auth.go` — full current `AuthService` implementation; the extraction source for FR-044. Key methods: `ValidateCredentials` (bcrypt-based), `CreateSession`/`createSessionWithFlags`, `ValidateSession`, `GetSession`, `DestroySession`, `runCleanupLoop`/`cleanupExpiredSessions`, `GenerateCSRFToken`/`ValidateCSRFToken`, `UpdatePassword`, `isDefaultCredentials`.
- `internal/adapter/gateway/cli/gateway.go` — existing `Set*Handler` wiring pattern (`SetAdminHandler`, `SetProfileHandler`, `SetBotHandler`, `SetMemoryHandler`, `SetConfigHandler`, `SetSkillHandler`) and the `IsMemoryCommand()` prefix-dispatch call site (line ~104) — the pattern new environment handlers and the new `/memories` (plural) dispatch should follow.
- `internal/adapter/gateway/cli/memory_commands.go` — `IsMemoryCommand()` implementation, the existing `/memory` (singular) admin-command prefix that FR-037/038's `/memories` (plural) must not collide with.
- `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` — existing per-user-scoped usecase services already consumed by the web adapter; the new CLI handlers call these directly, no duplicated business logic.
- `cmd/nuimanbot/main.go` (~line 1156) — current unconditional auto-admin `SetCurrentUser` call and (~line 1173) hardcoded `"cli_user"` chat-attribution placeholder — both replaced by this feature.

## API Documentation

[N/A — no external API integration; this feature is internal-architecture refactoring plus new CLI command surface consuming existing internal usecase interfaces.]

## Best Practices

Confirmed against the PRD's stated NFRs, resolved in `architecture.md`:
- Secure file permissions (`0600`) on the session-token file — carried through unchanged from the PRD/spec into `architecture.md` AD-2's concrete design.
- Credential handling — passwords are never persisted; only the post-authentication session record touches disk. `bcrypt` (already used by `auth.Service.ValidateCredentials`, cost 12) is reused as-is, no new hashing scheme introduced.
- Avoiding plaintext logging of secrets — the login flow must not `slog.Info`/log the raw password anywhere in the new CLI code path (mirrors the existing web `handleLogin`, which never logs `password`/`username` together with an outcome that would leak credential-guessing signal — see the Security NFR's "failed login attempts don't leak whether a username exists vs. a wrong password").
- Threat model for the persisted session file — explicitly decided in `architecture.md` AD-2, not left as a generic "follow best practice" gesture: the trust boundary is the OS user account (0600 permissions), and forging your own session file grants nothing beyond what running the binary as yourself already grants — see AD-2 for the full reasoning.

## Open Questions — Resolved During Pipeline Step 2 Review

All three carried-forward questions are now decided; none are open going into implementation.

- **Session-token expiry duration:** resolved to `24 * time.Hour`, matching `internal/adapter/web/auth.go`'s existing `sessionTimeout` constant exactly. No divergent CLI-specific value — see `architecture.md` AD-2 for the reasoning (kept simple: `auth.Service.sessionTimeout` becomes a single shared field once the extraction lands; no per-adapter override was justified).
- **CLI password entry UX:** masked input, via `golang.org/x/term.ReadPassword`. Checked `go.mod`/`go.sum` directly — `golang.org/x/term` is **not currently a dependency**, direct or indirect (only `golang.org/x/crypto v0.40.0` is present, and it does not pull in `x/term`). This needs to be added as a new direct dependency (`go get golang.org/x/term` + `go mod tidy`) during implementation of the login-prompt task in `tasks.md` Phase B — flagged so it isn't a surprise mid-implementation.
- **`/settings set network-mode` CLI exposure:** this question is **superseded by FR-043**, which already made the decision ("expose with caveat," matching the web UI's own admission) at spec-creation time — the PRD's Open Questions section simply wasn't reconciled with its own FR list. No further research needed; deleting this as a live open question. `/settings set network-mode`'s CLI help text must state the limitation explicitly, per FR-043 and the Risks table entry, unchanged from the original draft.

## Original Open Questions (superseded, kept for traceability)

- Session-token expiry duration is not pinned — propose a sensible default during spec/implementation planning.
- Exact CLI login UX for password entry (masked vs. plain) is left as an implementation detail for the spec phase.
- Whether `/settings set network-mode` from the CLI should be blocked entirely vs. exposed with a caveat — worth revisiting once/if the underlying web Settings limitation (FR-R11) is fixed in a future pass.

## References

- PRD: [cli-parity-for-nuimanbot-features-PRD.md](./cli-parity-for-nuimanbot-features-PRD.md)
- `spec.md` in this directory
