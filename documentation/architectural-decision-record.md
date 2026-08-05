# Architectural Decision Record

This is NuimanBot's running ADR log. Each entry is dated and numbered, and once
recorded is not edited to look better in hindsight — a decision that later
proves wrong gets a new entry that supersedes it, with a link back to the one
it replaces. See `documentation/product-summary.md` (executive overview),
`documentation/product-details.md` (requirements/workflows), and
`documentation/technical-details.md` (architecture) for the current, accurate
state of what's shipped; this file exists to preserve *why*, which those
documents intentionally don't carry.

Format per entry: Title, Status, Context, Decision, Consequences.

---

## ADR-001: Reuse the existing prompt-injection pattern set for tool-output validation

**Date:** 2026-08-02
**Status:** Accepted

### Context

NuimanBot already had a prompt-injection detector (`internal/usecase/security/input_validation.go`,
`detectPromptInjection`) wired to direct human chat input, matching against
~30 jailbreak/role/disclosure/output keyword patterns. The security-hardening
work (Part A) needed the same kind of detection applied to a different data
path: content the agent *fetches on its own behalf* (web pages, search
snippets, MCP responses) before that content re-enters the LLM's conversation.
The naive approach would be a second, output-specific detector with its own
pattern list.

### Decision

Extract the four keyword-pattern groups out of `DefaultInputValidator` into a
shared file, `internal/usecase/security/prompt_injection_patterns.go`
(`promptInjectionPatternGroups`, `defaultPromptInjectionPatterns()`,
`detectPromptInjectionPatterns(patterns, input) (bool, []string)`).
`DefaultInputValidator.detectPromptInjection` now delegates to this helper,
and the new `OutputValidator` (`output_validation.go`) uses the exact same
helper and pattern set — no new pattern categories were introduced for output
validation.

### Consequences

- Input-side and output-side injection detection can never drift apart
  (a pattern added/removed for one path is automatically reflected in the
  other) — this was the core motivation.
- The output path inherits every known weakness of the existing keyword-match
  approach (it catches known phrasing, not paraphrases or obfuscation) rather
  than getting an independently-tuned detector; mitigated by layering with the
  Part B structural guardrail and the Part C confirmation gate (ADR-003), not
  by a stronger single detector.
- A single shared pattern set means any future improvement to detection
  quality benefits both paths for free.

---

## ADR-002: Generalize the persona-memory-write confirmation pattern into `ConfirmationStore`

**Date:** 2026-08-02
**Status:** Accepted

### Context

The codebase already had a working, narrow confirmation precedent:
`memorywriter.go`'s persona-memory-write flow, which surfaces
`NeedsConfirmation`/`ConfirmationID` fields for a specific, single use case.
Part C needed a general-purpose "pause a side-effecting tool call and ask a
human yes/no" mechanism spanning many tools (`github.pr_merge`,
`coding_agent.yolo_mode`, MCP write-trust tools, etc.) and every chat gateway,
web admin UI, and the REST API.

### Decision

Generalize the existing pattern into a new `ConfirmationStore` interface
(`internal/usecase/security/confirmation_store.go`: `Create`/`Resolve`/`Get`/
`GetOpenByKey`/`ListPending`/`ExpireStale`) and a file-backed implementation
(`internal/infrastructure/security/confirmation_store.go`,
`FileConfirmationStore`), rather than inventing an unrelated, tool-loop-specific
mechanism or duplicating the memory-writer's narrower shape for every new
use site.

### Consequences

- One mechanism now backs every side-effecting confirmation surface: chat
  gateways (plain text and Slack/Telegram buttons), the web admin UI, and the
  REST API all read/write through the same `ConfirmationStore` contract.
- `GetOpenByKey` and `ListPending` were both added *after* the initial
  interface (for P5.5's reply-detection and P5.8's listing-UI needs
  respectively) — an additive-interface-method pattern that required updating
  every test-double implementer each time, a recurring cost of generalizing
  early.
- The file-backed implementation needed its own atomic-write helper rather
  than reusing `internal/infrastructure/storage.AtomicFileWriter`, because
  that package already imports `internal/infrastructure/security` (for
  `EncryptionService`) — reusing it would have created an import cycle.

---

## ADR-003: End the tool-calling turn synchronously; treat confirmation resolution as a fresh follow-up turn

**Date:** 2026-08-02 (Phase 5 / Part C)
**Status:** Accepted

### Context

When a gated tool call is attempted mid-conversation, something has to happen
to the in-flight tool-calling loop (`chat.Service.ProcessMessage`'s
`runToolLoop`, up to 5 iterations). Making the loop `await` a human response
asynchronously — i.e. suspending the in-progress turn until a reply arrives,
however long that takes — would require threading a
resumable-continuation/goroutine-parking model through the tool loop, and
would leave a turn "open" indefinitely across gateway reconnects, process
restarts, or a user who never replies.

### Decision

When a tool call resolves to `StatusPendingConfirmation`, the current turn
ends immediately (the pending-confirmation flag propagates via
`domain.ToolResult.Metadata`, the same mechanism Part A already used for
`injection_flagged`) and does not consume a `maxToolIterations` round.
`ChatService.ProcessMessage` checks `ConfirmationStore.GetOpenByKey(userID,
conversationID)` at the *start* of every subsequent incoming message from that
conversation to decide whether it's a resolving reply. An approving reply
re-invokes the original tool call directly with its original parameters and
feeds the result into a brand-new `runToolLoop` invocation (its own fresh
5-iteration budget) rather than resuming the original loop.

### Consequences

- No suspended/parked goroutine or continuation state survives across the gap
  between the pending-confirmation reply and the user's response — the
  pending state lives entirely in `ConfirmationStore`, which is durable and
  survives a process restart.
- Required a substantial refactor of the previously-313-line
  `ChatService.ProcessMessage` into `runToolLoop`/`finishTurn`/
  `finishPendingConfirmationTurn`/`saveTurnMessages`/`composeSystemPrompt`/
  `historyToMessages`, since the "approved" path genuinely needs a second,
  independent tool-loop invocation reaching the same turn-completion logic —
  this was necessary, not optional cleanup.
- `ProcessMessageStream` (the streaming reply path) was deliberately left
  untouched: it already falls back to an error if the LLM attempts a tool
  call, so Part C's confirmation gate has nothing to hook into there today. A
  future phase adding streaming tool-call support will need to revisit this.

---

## ADR-004: Serialize to one open confirmation per conversation

**Date:** 2026-08-02 (Phase 5 / Part C)
**Status:** Accepted

### Context

A conversation could, in principle, trigger multiple side-effecting tool
calls before the first one is resolved (e.g. the LLM proposes both a PR merge
and an issue close in the same turn, or across turns before the user
responds). Supporting several simultaneously-pending, independently
addressable confirmations would require a numbered/referenced UX (e.g. "reply
'yes 2' to approve the second request") across every gateway, the web UI, and
the REST API.

### Decision

`ConfirmationStore.Create` enforces at most one open confirmation per
`(UserID, ConversationID)`. A second side-effecting request while one is
pending is rejected outright with a message that the prior action must
resolve first, rather than queued or given its own identifier for the user to
disambiguate.

### Consequences

- Every gateway's yes/no UX (plain text, Slack Block Kit buttons, Telegram
  inline keyboard, web admin Approve/Deny, REST resolve) stays simple: "yes"
  or "no" is always unambiguous because there is never more than one thing to
  answer.
- A conversation that generates a second gated action while one is still
  pending gets a hard rejection, not a queue — this is a deliberate scope cut
  (numbered multi-pending-confirmation support is explicitly out-of-scope,
  see `spec.md`'s Out-of-Scope section), and would need to be revisited if
  usage patterns show this rejection happening often enough to be annoying.
- Concurrent-request races on the same key are handled by a per-`(UserID,
  ConversationID)` lock in `FileConfirmationStore`, so "exactly one `Create`
  succeeds" holds even under concurrent calls, not just logically in the
  single-threaded case.

---

## ADR-005: Resolve-then-validate-then-dial-the-resolved-IP for SSRF protection

**Date:** 2026-08-02 (Phase 4 / Part E)
**Status:** Accepted

### Context

`ValidateFetchURL` resolves a hostname and validates the resulting IP(s), but
Go's default `net.Dialer`/`http.Transport` independently re-resolves the same
hostname when it actually dials — including on every redirect hop. This opens
a classic DNS-rebinding TOCTOU window: an attacker's DNS server can answer
with a public, validation-passing IP on the first (validation) lookup and a
private/loopback IP a few milliseconds later on the second (dial) lookup.
Validating a hostname is not the same as validating the connection that's
actually opened.

### Decision

Three cooperating pieces in `internal/usecase/tool/common/ssrf_transport.go`:
(1) a context-carried pinned IP (`WithResolvedIP`/`resolvedIPFromContext`);
(2) `NewCheckRedirect`, the only place that performs the validate-and-pin
step, which resolves and validates each redirect hop's target via
`ResolveValidatedIP` and mutates the pending `*http.Request` in place
(`*req = *req.WithContext(...)` — reassigning a local variable would not
affect what the `http.Client`'s internal loop actually sends, since the
caller only observes mutations through the existing pointer); (3)
`pinnedDialContext`, which rewrites the dial address's host portion to the
pinned IP before calling the underlying dialer, so the underlying dialer is
handed a bare IP literal and never independently resolves anything. `req.URL`
and the `Host` header are left untouched, so TLS SNI and HTTP virtual-hosting
still target the original hostname — only the network-layer routing is
pinned.

### Consequences

- The dial for a validated redirect hop is guaranteed to connect to the
  exact address that was validated, closing the DNS-rebinding TOCTOU window
  for redirects.
- **Scope is deliberately limited to redirect hops, not the initial request.**
  `validateURL`/`validateSource` (the `Execute()`-level, initial-request
  check) call `ValidateFetchURL` for validation only — they do not pin an IP
  for the subsequent `httpClient.Do(req)` call. The same small,
  standard "validate the initial URL, then dial it" TOCTOU window that
  essentially every SSRF-safe HTTP client built on `net/http` accepts remains
  for the first hop. Closing it completely would require either a custom
  DNS-to-dial pipeline for every request (including ones with no redirect at
  all) or abandoning the stock `net/http` client/transport entirely — judged
  disproportionate to this feature's scope.
- Testing "redirect to an *allowed* target succeeds" without real internet
  access required redirecting to a public-range IP *literal* (validated with
  zero DNS lookups) and faking only the dial via a test-only stub — proving
  the real, unmocked validation and pinning logic without any outbound
  network dependency.

---

## ADR-006: Dynamic `mcp:*` trust-based permission resolution via a `TrustClassifiedTool` interface

**Date:** 2026-08-02 (Phase 6 / Part F)
**Status:** Accepted

### Context

Part D's `ToolPermissions` (`internal/usecase/tool/permissions.go`) is a
static `map[string]domain.Role` populated at compile time. MCP tool names
(`mcp:<server>:<tool>`) are only known at runtime — `mcp.json` is read at
startup, and even then only the specific tool names an MCP server reports via
`tools/list` are known — so a static map entry for an MCP tool is structurally
impossible.

### Decision

`internal/adapter/mcp.MCPToolAdapter` gained a `trust string` field and
`TrustLevel() string` method, populated at bridge-construction time from
`infra.ResolvedToolTrust(entry, toolName)`. `internal/usecase/tool` defines a
new interface, `TrustClassifiedTool` (`TrustLevel() string`) — deliberately
*not* added to `domain.Tool` itself, since that interface is implemented by
every tool, most of which have no concept of trust. `Service.resolveRequiredRole`
and `Service.enforceRulesAndConfirmation` both check `isMCPTool(toolName)`
(a `"mcp:"` prefix match) and, when true, resolve trust via a
`s.registry.Get(toolName)` call followed by a `t.(TrustClassifiedTool)` type
assertion — reusing the `ToolRegistry` reference `Service` already holds,
rather than requiring new MCP-specific wiring through `Service`'s constructor.
This dynamic-lookup path sits alongside, not instead of, the static map
lookup.

### Consequences

- Fail-closed at every step: a nil registry, a `Get` error, a tool not
  implementing `TrustClassifiedTool`, or an unrecognized `TrustLevel()` value
  all resolve to `TrustUnknown`, which RBAC treats as `RoleAdmin`-equivalent
  and confirmation treats as required — a permission check must never
  interpret an unrecognized signal as "safe."
- Trust-level constants (`TrustReadOnly`/`TrustWrite`/`TrustUnknown`) are
  deliberately duplicated as plain strings in both `internal/infrastructure/mcp`
  and `internal/usecase/tool`, rather than sharing one Go type across the
  layers — consistent with this codebase's existing pattern (e.g.
  `config.ToolOutputValidationConfig.Action` as a plain string translated to
  `security.ValidationAction` only at the DI layer) of keeping
  `internal/infrastructure` and `internal/usecase` decoupled at the type
  level even where they agree on a shared string "wire format."
- `ResolvedToolTrust` re-normalizes defensively (not just once at
  `LoadMCPConfig` time), so a directly-constructed `MCPServerEntry` (as tests
  do, bypassing `LoadMCPConfig`) still resolves correctly instead of silently
  returning an un-normalized empty string.

---

## ADR-007: Breaking RBAC-default change for `github`/`coding_agent` to admin-only, with a config escape hatch

**Date:** 2026-08-02 (Phase 3 / Part D)
**Status:** Accepted

### Context

Auditing the tool registry (`cmd/nuimanbot/main.go`'s `registerBuiltInTools`/
`registerDeveloperProductivityTools`) against `ToolPermissions` found that
`github` and `coding_agent` had no explicit entry and silently fell through to
`DefaultToolPermission` (`RoleUser`) — meaning any registered user, not just
admins, could invoke GitHub write actions (`pr_merge`, `issue_create`, etc.)
and the external coding agent (which has shell access), despite this being
exactly the kind of high-privilege, side-effecting capability an injected
instruction would want to reach.

### Decision

`github` and `coding_agent` move to `RoleAdmin` by default in
`ToolPermissions`. `github`'s check is additionally action-aware
(`resolveRequiredRole`/`githubActionRole`): read actions (`issue_list`,
`issue_view`, `pr_list`, `pr_view`, `repo_view`) stay `RoleUser`; write actions
and any unrecognized/missing action resolve to `RoleAdmin` (fail closed —
an action the classifier doesn't recognize is treated as a write, not a
read). A new config-level `tools.permissions` map (`internal/config`) lets an
operator override any tool's effective role without a code change, at
whole-tool granularity (not per-action), specifically for deployments that
need to restore the old, more permissive behavior.

### Consequences

- This is an intentional, security-motivated **breaking change**: deployments
  where non-admin users currently invoke `github` writes or `coding_agent`
  will see access denied until an operator sets the `tools.permissions`
  override or grants the affected users Admin. Called out prominently in
  `README.md`, `documentation/product-summary.md`, `documentation/product-details.md`,
  and `support_docs/security-hardening-guide.md`.
- The override is whole-tool, not whole-tool-and-action — an operator who
  wants `github` reads open to `RoleUser` but writes still admin-only already
  gets that automatically from the default action-aware split; the override
  exists specifically for the coarser "revert this whole tool" case.
- A malformed override value (not exactly `guest`/`user`/`admin`,
  case-insensitive) is logged and ignored, falling through to the next
  precedence level, rather than silently granting or denying based on a typo.

---

## ADR-008: Known, accepted gap — live chat tool-calling does not enforce role-based RBAC

**Date:** 2026-08-03 (identified during Phase 5, re-confirmed through Phase 7)
**Status:** Accepted / deferred (not fixed in this feature)

### Context

`internal/usecase/tool.Service` exposes two entry points: `Execute(ctx,
toolName, params)` (no RBAC check at all) and `ExecuteWithUser(ctx, user,
conversationID, toolName, params)` (performs `checkPermission`/
`resolveRequiredRole` — the actual RBAC enforcement path — plus the
confirmation gate). Auditing call sites found `ExecuteWithUser` has **zero
production callers** anywhere in the codebase — only test files call it.
`cmd/nuimanbot/main.go` wires `chatService := chat.NewService(llmService,
conversationAdapter, toolExecutionService, securityService)`, and
`chat.ToolExecutionService`'s interface only declares `Execute`. This means
Part D's RBAC policy (`ToolPermissions`, the action-aware `github` split, the
MCP trust-based resolution) is fully defined, unit-tested, and CI-guarded,
but is **not enforced end-to-end for a tool call made during a real
conversation** — a pre-existing gap that predates this feature's Phase 5, not
introduced by it. A second, related gap surfaced later (Phase 7): `Service.ListTools`
carries a pre-existing `// TODO: Implement user-specific tool filtering` and
returns every registered tool unfiltered by role — meaning the LLM is offered
every admin-only tool (`github`, `coding_agent`, `executor`) as a callable
option in every conversation for every user, not merely that a chosen call
goes unchecked.

### Decision

Do not close this gap as part of this feature. Fully closing it means
resolving a role-bearing `*domain.User` from `incomingMsg.PlatformUID` inside
`ChatService`'s hot path — a new `UserRepository` dependency, plus a
default-role-assignment policy for first-seen platform users — which is
substantially larger in scope than Parts A–G and arguably belongs to a
dedicated identity-unification effort, not a bolt-on to this hardening pass.

Instead, made the **confirmation gate** (Part C, ADR-002/ADR-003) independently
functional despite this gap: `security.ConfirmationIdentity`/
`WithConfirmationIdentity`/`ConfirmationIdentityFromContext`
(`internal/usecase/security/confirmation_context.go`) carry a context-scoped
`(UserID, ConversationID)` pair (mirroring the `WithResolvedIP` precedent from
ADR-005). `tool.Service.Execute` — the method the live chat loop actually
calls — checks for this identity and, if present, runs the same
`enforceRulesAndConfirmation` helper `ExecuteWithUser` uses, applying
`RulesEnforcer`/config-default confirmation gating for that call. This does
**not** run the role-based `checkPermission` step, which still requires a
full `*domain.User`.

### Consequences

- Part C's confirmation gate is genuinely live for real conversations today
  (not just reachable via mocks in unit tests) — a user really is asked to
  approve `github.pr_merge`/`coding_agent.yolo_mode`/write-trust MCP tools in
  production chat.
- Full role-based RBAC (`ToolPermissions`, the `github` action-aware split,
  MCP trust-based role resolution) is **not** enforced in live chat
  conversations — only reachable today via `ExecuteWithUser`, which nothing
  in production calls. A user without Admin role who is *not* caught by a
  confirmation requirement can still have a tool executed on their behalf
  that RBAC, if enforced, would have denied.
- `ListTools` returns every registered tool to every caller regardless of
  role — the LLM is offered admin-only tools as callable options for
  non-admin users, even though attempting to *invoke* one of those tools
  through `ExecuteWithUser` (were it called) would be denied.
- Both gaps share the same root cause (no role-bearing `*domain.User`
  resolved on the live chat path) and the same fix — closing the RBAC gap in
  a future phase would very likely let confirmation gating switch to the full
  `ExecuteWithUser` path for free, and would naturally motivate adding
  role-based filtering to `ListTools` at the same time.
- This is documented, not swept under the rug, in `README.md`'s "Known
  Limitation" section, `documentation/product-summary.md`'s "Known limitation
  — RBAC in live chat" paragraph, `documentation/product-details.md`'s FR-002/
  SC-004, and `documentation/technical-details.md`'s Tool Execution Service
  section — all corrected for accuracy in the Phase 7 documentation-parity
  pass. Follow-up: track as a dedicated future spec (identity unification +
  `ExecuteWithUser` wiring + `ListTools` role filtering), not a quick patch.

**Update (2026-08-03):** This gap was closed by commit `cecf931`
("FR-001/FR-002 — fix RBAC bypass in live chat (P0)"), on this same feature
branch. Summary of the fix: an already-production-wired
`domain.UserProfileRepository`/`profile.Service` (file-backed,
`GetProfileByPlatformID(platform, uid) → Role`) was discovered and used to
resolve a real role-bearing `*domain.User` for each incoming chat message,
via a new `chat.UserResolver` interface (`SetUserResolver`/`resolveUser` on
`chat.Service`), which fails closed to `domain.RoleGuest` when a platform
identity can't be resolved. Both of `chat.Service`'s tool-execution call
sites — `ProcessMessage`'s main tool-calling loop and the
confirmation-approval re-invocation path — were changed to call
`ExecuteWithUser` instead of `Execute`, closing the first gap described
above. `tool.Service.ListTools` was fixed to filter the returned tool list
by the resolved user's role, closing the second gap. `cmd/nuimanbot/main.go`
now wires a `profileRoleResolver` adapter connecting `profile.Service` to
`chat.UserResolver`. A regression test,
`TestProcessMessage_RBAC_NonAdminCannotInvokeGitHubPRMerge`
(`internal/usecase/chat/rbac_test.go`), proves a non-admin chat user's
attempt to invoke `github.pr_merge` is now denied at the RBAC layer, not
merely gated by confirmation. The original Decision/Consequences above are
left as written to preserve the historical record of why this was initially
deferred; they no longer describe the current state of the code — see
`README.md`, `documentation/product-summary.md`,
`documentation/product-details.md` (FR-002/SC-004), and
`documentation/technical-details.md` (Tool Execution Service section) for
the corrected, current documentation.

**Update (2026-08-04):** While merging this feature branch onto `main`, an
*independently developed* fix for this exact same P0 finding was discovered
already merged there — the separately-delivered Buzz gateway feature (PR #5)
had its own reviewer flag the identical gap (its spec's research.md Q6) and
fix it in that PR's own Phase 3 (commits `88e5065`/`e8c6ccc`), using a
different mechanism: a persisted `domain.User`/`UserService`
(`GetUserByPlatformUID`/`CreateUser`, backed by a dedicated
`domain_users.json`, distinct from `profile.Service`'s `users.json`) rather
than a read-only role lookup against the existing profile repository. The
reconciliation adopted Buzz's persisted-user model as canonical (it was
already shipped and other code — the Buzz gateway itself — depended on it)
and removed this ADR's `chat.UserResolver`/`profileRoleResolver` mechanism
entirely, folding this feature's additional requirements on top of it:
`ExecuteWithUser`/`ListTools` gained a `conversationID` parameter Buzz's
version didn't need (required for Part C's confirmation gate), the
action-aware `github` split and MCP trust-classification logic from this
feature's `resolveRequiredRole` were carried over unchanged (Buzz's fix
never touched `checkPermission`'s body), and — critically — confirmation-reply
detection (`tryResolveConfirmationReply`) was changed to key its
`ConfirmationStore` lookup on the resolved user's persisted ID rather than
raw `PlatformUID`, since `ExecuteWithUser` now creates confirmations keyed on
that same persisted ID (a real UUID under the adopted model, not
interchangeable with `PlatformUID` the way this ADR's original mechanism
assumed) — this specific mismatch would have silently broken every pending
confirmation's plain-text yes/no reply detection had it not been caught
during reconciliation. See `internal/usecase/chat/rbac_test.go` for both
sets of regression tests, now coexisting in one file (Buzz's cross-platform
tests plus this feature's action-aware/MCP-trust tests, both passing against
the unified model).

---

## ADR-009: Extend `ConversationRepository` for Chats rather than a new entity

**Date:** 2026-08-05 (spec review, `specs/260805-nuimanbot-extend-context-and-ui`)
**Status:** Accepted

### Context

The web admin needed a Chats environment (FR-011–FR-016): lightweight,
directory-less conversations, auto-named from their first message,
independently retained, exportable, and deletable. The codebase already had
`domain.Conversation`/`ConversationRepository` plus
`internal/infrastructure/storage/file_conversation_repository.go`
(`FileConversationRepository`) — a per-user, file-based, atomic-write
conversation store already tracking `UserID`, `CreatedAt`/`UpdatedAt`,
message count, and last-message snippet — backing the CLI/Telegram/Slack/Buzz
gateways' conversation history. A new `Chat` entity would have duplicated
most of `Conversation`'s shape for no functional gain.

### Decision

Extend `Conversation`/`ConversationRepository` with the fields Chats need
(auto-generated `Name`, `RetentionPolicy`) rather than introducing a
parallel `Chat` domain entity. `internal/usecase/chats.Service` wraps
`ConversationRepository` directly, and `ExportChat` reuses the pre-existing
`chat.Service.ExportConversation` (JSON/Markdown) rather than building new
export logic — a reusable asset found during spec review, not referenced in
the original PRD.

### Consequences

- Chats and CLI/Telegram/Slack/Buzz gateway conversations share one
  underlying entity and one file-based store — no data model fork, no new
  repository to keep in sync with the existing one.
- The web Chats environment inherits `ConversationRepository`'s existing
  shape but **not** any wiring to the LLM — `chats.Service.AppendUserMessage`
  only appends a user-role message via `ConversationRepository.AppendMessage`
  and returns. Extending an existing *storage* entity did not, by itself,
  connect the web UI to `chat.Service.ProcessMessage` or any other
  agent-invocation path — that remains unbuilt (see
  `documentation/product-summary.md`'s "Persistent Agent Workspace — In
  Progress" and `documentation/product-details.md`'s FR-025).
- `FileConversationRepository`'s pre-existing raw-`filepath.Join` path
  construction (predating this feature) was not brought onto the new
  `fsguard` path-confinement pattern (ADR-013) — flagged as a follow-up,
  not fixed here, since it was out of scope for a "reuse, don't rebuild"
  decision.

---

## ADR-010: `robfig/cron/v3` for Chore scheduling

**Date:** 2026-08-05 (spec review)
**Status:** Accepted

### Context

Chores (FR-031–FR-038) need cron-style recurrence: preset shorthands
(hourly/daily/weekly/monthly) plus a raw cron expression field, skip-if-
still-running semantics (FR-035), and a persisted `NextFireTime` that
survives a restart without missing or double-firing a scheduled window. No
existing scheduler, cron parser, or job-queue infrastructure existed in the
codebase.

### Decision

Add `github.com/robfig/cron/v3` as a new `go.mod` dependency (the only new
third-party dependency this feature introduces) rather than writing an
in-house cron parser. `internal/infrastructure/scheduler/cron.go` wraps it
behind two package functions, `ValidateCronExpression` and `NextFireTime`,
so no other package in the codebase imports `robfig/cron/v3` directly —
`internal/domain` and `internal/usecase/chores` depend only on
`chores.ScheduleEvaluator` (an interface `cmd/nuimanbot`'s
`scheduleEvaluatorAdapter` implements against the wrapper).

### Consequences

- `Schedule.Next(time.Time) time.Time` directly provides what FR-035
  (skip-if-still-running) and the restart-durability NFR both need, without
  reimplementing well-tested cron-grammar parsing.
- Keeping the dependency behind a two-function wrapper means a future
  replacement (a different cron library, or an in-house parser) only touches
  `cron.go` and its adapter — no ripple into `domain` or `usecase`.
- `go mod tidy` reclassified this dependency from indirect to direct once
  `internal/infrastructure/scheduler` imported it, which is expected and
  was verified as part of this feature's quality-gate pass, not a leftover
  from `gorilla/websocket` (which was already a direct dependency via the
  Buzz gateway).

---

## ADR-011: WebSocket over polling for near-real-time run/notification updates

**Date:** 2026-08-05 (spec review)
**Status:** Accepted

### Context

Job/Chore run status, log output, and the History notification badge need to
update without a manual page refresh while a run is active (Performance
NFR). The two realistic options were client-side polling (simple, but adds
either latency or request volume depending on interval) and a push
transport. `github.com/gorilla/websocket` was already a `go.mod` dependency,
used by `internal/infrastructure/nostr/client.go` for the Buzz gateway's
relay connections — but never inside `internal/adapter/web`.

### Decision

Use `gorilla/websocket` for a per-user push transport (`Hub`,
`internal/adapter/web/websocket_handler.go`) rather than polling, precisely
because the dependency was already present and battle-tested elsewhere in
the codebase — adopting it for a second use case cost zero new dependencies.
Connection/subscription model: one WebSocket connection per browser tab,
subscribed server-side to a per-user broadcast channel (not per-run); a
disconnect is handled by simple client-driven backoff plus a full state
resync over the existing HTTP API on reconnect — no server-side replay
buffer (spec.md Edge Case #5).

### Consequences

- The transport itself — handshake, per-user channel isolation, bounded
  per-client send buffers so one slow client can't stall delivery to
  others — is implemented and verified under `-race`.
- Choosing WebSocket over polling was a bet that paid off on the
  *transport* side but not yet on the *product* side: **no browser-side
  JavaScript consumes this feed yet**. The server pushes every `RunEvent`
  correctly; nothing in the shipped templates subscribes to `/ws` or updates
  the DOM from it. This is the single largest gap between "architecturally
  correct" and "user-visible" in this feature — see
  `documentation/product-summary.md`'s "Persistent Agent Workspace — In
  Progress" for the full list this sits alongside.
- Treating this as net-new engineering (not a reused pattern) was the right
  call in hindsight: Buzz's Nostr usage of `gorilla/websocket` is a relay
  *client*; this feature needed a connection-accepting *server* with
  per-user fan-out, a structurally different problem the shared dependency
  didn't already solve.

---

## ADR-012: File-based `AtomicFileWriter` persistence over introducing a database

**Date:** 2026-08-05 (spec review)
**Status:** Accepted

### Context

Project, Job, Chore, and Run records, plus the worker pool's FIFO queue
state, all need crash-safe persistence meeting the Reliability NFR: a
restart must not lose a queued Job, drop an in-flight run's record, or cause
a Chore to miss its next fire time. The codebase already had
`internal/infrastructure/storage/atomic_file_writer.go`
(`AtomicFileWriter`: temp-file + rename atomic write; `FileLock`:
flock-based exclusive locking), used for `users.json`/`bots.json` and the
existing `FileConversationRepository`. The alternative was introducing
SQLite or another embedded database specifically for this feature.

### Decision

Use `AtomicFileWriter` + `FileLock` for every new entity (`FileProjectRepository`,
`FileJobRepository`, `FileChoreRepository`, `FileRunRepository`) and for the
worker pool's `Queue` — no new persistence mechanism, no write-ahead log.

### Consequences

- Run/queue volume is bounded by a single-process, single-machine worker
  pool, not a distributed system — the same atomic-rename + flock guarantee
  (a write either lands whole or not at all) that already protects
  `users.json`/`bots.json` is exactly what queue/run-state persistence
  needs, and this feature never needed to reach for anything stronger.
- Consistency across the whole codebase: every new repository looks and
  behaves like every existing file-based repository, so the same operational
  runbook (backup a directory, inspect a JSON file) applies to the new
  entities without a new set of tools or procedures.
- This is a scale ceiling, not a defect: if a future deployment needs
  multi-process worker pools or very high run-volume throughput, this
  decision should be revisited — profiling data showing write-frequency
  contention was the explicitly named trigger for reconsidering it (spec.md),
  and no such profiling has been done, because no such contention has been
  observed at this feature's current scale.

---

## ADR-013: `fsguard` path-confinement helper for Project/Job/Chore/Run file operations

**Date:** 2026-08-05 (spec review; hardening fix same day during Phase 6/7)
**Status:** Accepted

### Context

Every Job/Chore/Project file operation resolves a user- or agent-supplied
relative path (a Project's output/hidden directories, a Job/Chore's hidden
directory, a Run's artifact directory) against a base directory. Spec review
verified no reusable path-confinement helper existed:
`internal/infrastructure/preprocess.CommandSandbox` sandboxes *command
execution* (a shell whitelist + timeout), not filesystem path containment;
`internal/config.FetchSecurityConfig` guards SSRF (network targets), not
local paths. Building this net-new was treated as required hardening work,
not optional (spec.md's Risks table lists Project directory sandboxing as a
High-severity risk).

### Decision

Build a single, dedicated package, `internal/infrastructure/fsguard`, with
one function every Project/Job/Chore/Run file operation must call:
`ResolveWithin(baseDir, relPath string) (string, error)`. It rejects
absolute `relPath` values, any lexical `..`-escape above `baseDir`, and NUL
bytes; it performs no I/O and does not resolve symlinks (a documented,
deliberate scope limit for callers to handle separately if an untrusted
`baseDir` could itself be a symlink).

### Consequences

- `fsguard`'s own unit tests (adversarial: traversal, absolute paths, NUL
  bytes, sibling-directory prefix confusion) were solid from Phase 2 onward.
  **But a defense-in-depth gap surfaced later**, during Phase 7's own
  verification pass: `FileJobRepository`/`FileChoreRepository`/
  `FileProjectRepository`/`FileRunRepository` had all been built with raw
  `filepath.Join(userDir, id+".json")` for their record (and, for Run, log)
  paths — never routed through `fsguard.ResolveWithin`, despite the
  package's own doc comment mandating it. A crafted ID like
  `"../../../../etc/passwd"` was confirmed, in isolation, to read an
  arbitrary file off disk and return it as a valid record.
- This was **not** a confirmed live HTTP exploit — `net/http`'s `ServeMux`
  happens to redirect literal `..` path segments away before routing ever
  reaches a handler, and each handler's path parsing only ever extracts a
  single non-slash segment as the ID — but that was incidental behavior, not
  a deliberate control, so the gap was real defense-in-depth hardening, not
  a false alarm. Fixed by routing all four repositories' path construction
  through `fsguard.ResolveWithin`, mapping any resolution failure uniformly
  to `domain.ErrNotFound` (never disclosing more than "not accessible to
  you," consistent with SC-007's IDOR posture). Adversarial tests were added
  per repository: a table of traversal strings against Get/Delete
  (/AppendLog for Run), plus a concrete plant-one-user's-record,
  craft-another-user's-ID scenario per repository.
- The lesson generalizes beyond this feature: "the helper is safe" and
  "every call site actually uses the helper" are two different claims, and
  only testing the former does not verify the latter. `FileConversationRepository`
  (pre-existing, backs Chats, predates this feature) has the identical
  raw-`filepath.Join` shape on `convID` and was deliberately left unfixed —
  a known, explicitly flagged follow-up, not an oversight repeated blindly.

---

## ADR-014: `StubExecutor` as an interim, non-agent-invoking `Executor` implementation

**Date:** 2026-08-05 (implementation, Phase 3 of `specs/260805-nuimanbot-extend-context-and-ui`)
**Status:** Accepted / superseded-by-future-work (not a permanent design)

### Context

The worker pool (ADR-012's `Queue` + `WorkerPool`) needs something to
actually execute a dequeued `RunRequest`. A real implementation means
wiring a Job/Chore's `Description`/`JOB-DESCRIPTION.md` into the existing
agent/tool-calling machinery (`internal/usecase/chat` and friends) — a
substantial integration (resolving the right LLM provider/model, mapping
`WorkingDirectory` into the agent's tool sandbox, streaming log output back
per line, handling provider/LLM failures per spec.md Edge Case #13) that was
judged too large to land in the same pass as the queueing/scheduling/
persistence/WebSocket/network-access infrastructure, without leaving the
rest of that infrastructure completely undemonstrated (every run stuck at
`Queued` forever, no History content, no WebSocket events to test against).

### Decision

Ship `internal/infrastructure/scheduler.StubExecutor`, a functional,
non-agent-invoking `Executor`: it drives a `Run` through a real
`Queued` → `Running` → `Completed`/`Failed` lifecycle, appends real log
lines, and writes a real `RESULTS.md` (fsguard-confined) whose content
explicitly states "no agent/LLM invocation occurred" — never a silent or
disguised placeholder. Wire it as the only `Executor` implementation in
`cmd/nuimanbot/extended_context.go` for this pass.

### Consequences

- Every other piece of this feature — History, the notification badge, the
  WebSocket push transport, worker-pool concurrency/FIFO bookkeeping,
  restart-durability — has a genuine, testable, end-to-end execution path
  today, rather than being validated only against a mock.
- **This is the single most consequential scope cut in the whole feature.**
  Jobs and Chores queue and "run" correctly, but produce no real agent work
  product. Every documentation layer must state this plainly and avoid
  implying Jobs/Chores do real work today — `documentation/product-summary.md`,
  `documentation/product-details.md` (FR-027/FR-028), and
  `documentation/technical-details.md`'s Persistent Agent Workspace section
  all do so explicitly, per this ADR.
- The replacement path is a clean drop-in: nothing in `internal/infrastructure/scheduler`
  depends on `StubExecutor` specifically, only on the `Executor` interface —
  a future agent-invoking `Executor` can be substituted in
  `wireExtendedContextEnvironments` with no change to `Queue`, `WorkerPool`,
  or `ChoreScheduler`.
- The same "infrastructure real, product value deferred" pattern applies to
  the web Chats environment (ADR-009's consequences) and to the per-Job/
  Chore/Run/Memories chat interfaces (not built this pass) — all four gaps
  share one root cause: no piece of this feature invokes
  `internal/usecase/chat`'s agent/tool-calling loop. Closing that
  end-to-end is the natural next-phase scope, tracked in
  `documentation/product-details.md`'s "Post-Feature: Persistent Agent
  Workspace — Remaining Work."
