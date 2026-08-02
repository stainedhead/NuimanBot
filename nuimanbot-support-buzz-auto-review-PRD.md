# Code and Design Review: NuimanBot Support for Buzz

**Reviewed branch:** `feat/nuimanbot-support-buzz`
**Compared against:** `main` (merge-base `ed12c3c`)
**Spec reference:** `specs/260802-nuimanbot-support-buzz/spec.md`
**Review date:** 2026-08-02
**Review type:** Documentation-only (no fixes applied in this pass — see `specs/260802-nuimanbot-support-buzz/` follow-up phases for remediation)

---

## Executive Summary

This branch adds a complete 3-phase Buzz (Nostr-based multi-agent chat) gateway to NuimanBot, plus a cross-cutting RBAC-enforcement fix (`ExecuteWithUser` + role-filtered `ListTools()`) applied to all four platforms, plus a standalone encryption-key bugfix, plus new product and user documentation. The review covered Clean Architecture layering, TDD discipline, type safety/concurrency, code quality and dead-code, security (signature verification, RBAC, key handling, input validation), observability, and documentation accuracy.

**Overall assessment: solid, mergeable implementation with no security holes and no broken build.** Every audited security dimension — signature verification across all three consumed Nostr event kinds, RBAC bypass paths, CLI `RoleAdmin` spoofing resistance, private-key leakage, and input validation — came back clean, verified against actual code rather than taken on faith. Clean Architecture boundaries hold: `internal/infrastructure/nostr/` does not leak into `domain`/`usecase`, and `internal/adapter/gateway/buzz/` depends only inward. Test coverage is unusually strong for the core protocol logic (real signed events, real in-process WebSocket relays, `-race`-clean concurrency tests), not shallow or over-mocked.

The findings that do exist cluster in four places: (1) a genuine nil-pointer panic risk in `Gateway.Send()` that breaks from the established Slack-gateway guard pattern, plus untested lifecycle methods (`Start()`/`Stop()`); (2) two "declared but dead" items — the previously-flagged `buzz_relay_connections` metric, and a newly-found dead `NIP05` config field; (3) a real, though pre-existing and not Buzz-specific, observability gap where RBAC-denial and rate-limit metrics are declared but never incremented anywhere in the codebase; (4) one misleading line in the new user-facing documentation describing an environment-variable override that does not exist in the config loader. No P0 (blocker) findings were identified.

**Totals: 0 P0, 7 P1, 9 P2 — 16 findings.**

---

## Dimensions Reviewed With No Findings (explicitly confirmed clean)

Per review scope, these areas were specifically targeted and came back with **no P0/P1 issues** — stated explicitly rather than omitted:

- **Signature verification for all consumed kinds (kind:9, kind:9000, kind:10100):** `internal/adapter/gateway/buzz/gateway.go` `processEvent()` is the single choke point for every inbound event regardless of kind — dedupe → `nostr.Verify()` → only then does kind-based dispatch occur (to either `domain.IncomingMessage` construction or `agentCache` update). No code path reads event content/tags before verification. The team's "hardened in P2.3" claim was verified against the code, not assumed. See FR-012 for one related test-coverage gap.
- **RBAC bypass paths:** `tool.Service.ExecuteWithUser()` is the only path that reaches chat-triggered tool execution, for all four platforms; `tool.Service.Execute()` (unchecked) has no live production caller. `ListTools()` is genuinely role-filtered (prior `TODO` at `service.go:261` is resolved). Unknown roles fail closed (`Level() == -1`); user-lookup errors abort rather than defaulting to elevated access.
- **CLI `RoleAdmin` special-case spoofing:** The deliberate CLI-defaults-to-admin decision is implemented safely. `Platform` is hardcoded per gateway at construction time (`domain.PlatformBuzz` for Buzz, `domain.PlatformCLI` for CLI) — never derived from message/event content — so a network-facing Buzz message cannot be misclassified as CLI-originated to inherit `RoleAdmin`.
- **Vault/keygen private-key leakage:** No error message, log line, or panic anywhere in `internal/infrastructure/crypto/` or `internal/infrastructure/nostr/` embeds key/seed bytes. Config loading explicitly excludes `private_key` from generic struct-dump paths.
- **Input validation (`ValidateInput`):** Runs unconditionally as step 1 of `ChatService.ProcessMessage`, shared identically by all four gateways including Buzz — no Buzz-specific shortcut exists.
- **Clean Architecture layering:** `domain`/`usecase` have zero imports of `infrastructure/nostr` or any adapter package. `internal/adapter/gateway/buzz/` never imports another adapter package. `chat.UserService` is correctly defined as a consumer-side interface in the usecase layer. (One P2 style inconsistency noted at FR-011.)
- **Documentation accuracy (structural/protocol claims):** Of 11 factual claims cross-checked in `documentation/technical-details.md` and `support_docs/buzz-guide.md` against the actual code (config field names, event-kind semantics, loop-guard thresholds, vault key precedence, metrics list, library choice, RBAC/security claims), 10 were confirmed accurate, including the docs' own honest disclosure of the `buzz_relay_connections` gap. Only one doc claim (FR-007) was found to be actively wrong.

---

## Findings

### P1 — High priority, should be fixed before merge

#### FR-001: `Gateway.Send()` has no nil-guard on `g.client`, risking a nil-pointer panic

**File:** `internal/adapter/gateway/buzz/gateway.go:176-193`

`Send()` calls `g.client.Publish(...)` with no check that `g.client` is non-nil. If `Send()` is invoked before `Start()` has completed, or after `Stop()`/a failed `Start()`, this panics. This is a regression relative to the established pattern in this codebase: `internal/adapter/gateway/slack/gateway.go:84-86` explicitly guards with `if g.client == nil { return fmt.Errorf("Slack client not initialized") }` before use. Buzz's `Send()` omits the equivalent guard.

**Acceptance criteria:**
- `Gateway.Send()` returns a descriptive error (not a panic) when called with a nil/uninitialized `g.client`.
- A test exists that calls `Send()` on a `Gateway` that was constructed but never `Start()`ed (or was `Stop()`ped), asserting a graceful error return rather than a panic.

#### FR-002: `Gateway.Stop()` has 0% test coverage

**File:** `internal/adapter/gateway/buzz/gateway.go:162`

No test in `gateway_test.go` calls `gw.Stop()`. Connection-lifecycle shutdown — including whatever cleanup, cancellation, or resource release `Stop()` performs — is completely unverified by the test suite.

**Acceptance criteria:**
- At least one test calls `Stop()` on a started `Gateway` and asserts the expected post-stop state (e.g., context cancelled, no further events delivered, resources released, idempotent on double-`Stop()` if that's a supported call pattern).

#### FR-003: `Gateway.Start()` error/guard paths are untested

**File:** `internal/adapter/gateway/buzz/gateway.go:82-100`

Three distinct error paths in `Start()` have no corresponding test (confirmed by grepping `gateway_test.go` for the literal error strings — zero matches):
- Missing-relay guard: `"Buzz gateway requires at least one relay"` (line 84)
- Missing-private-key guard: `"Buzz private key is required"` (line 88)
- `nostr.Client.Start` failure wrap (line 100)

**Acceptance criteria:**
- A test exists for each of the three error paths above, asserting `Start()` returns the expected error and does not leave the gateway in a partially-initialized state that a subsequent `Send()`/`Stop()` call could panic on (ties to FR-001).

#### FR-004: `buzz_relay_connections` Prometheus gauge is declared but never set (confirmed known gap)

**Files:** `internal/infrastructure/metrics/prometheus.go:244-250` (declaration); `internal/infrastructure/nostr/client.go` (missing wiring)

Confirmed: `BuzzRelayConnections` is declared as a `GaugeVec`, but a repo-wide search finds zero `.Set()`/`.Inc()`/`.Dec()`/`.WithLabelValues()` calls against it anywhere. `internal/infrastructure/nostr/client.go` does not import the `metrics` package at all; its connection bookkeeping (`registerConn`/`unregisterConn`, `client.go:195-210`) and `ConnectedRelayCount()` (`client.go:214-218`) never touch the metric. Notably, `ConnectedRelayCount()` itself has no production caller — it is used only in tests (`gateway_test.go:456,459`, `client_test.go:192,195`) — which is further evidence this wiring was planned but never finished. The gauge sits at zero cardinality (no series exported at all, not even a stale `0`), which is more confusing for an operator than a stale value would be. This gap is honestly disclosed in `documentation/technical-details.md:730,1266-1268`, which reduces its severity as a "surprise" but does not change that it's an incomplete implementation of a stated spec requirement (spec.md's Observability section lists all four metrics as in-scope for this branch).

**Acceptance criteria:**
- `registerConn`/`unregisterConn` (or equivalent connect/disconnect points) call `BuzzRelayConnections.WithLabelValues(relayURL, ...).Set(1)` / `.Set(0)` (or an Inc/Dec pairing) so the gauge reflects real relay connection state.
- Because `internal/infrastructure/nostr` currently declares itself free of domain/config/adapter imports (see `event.go:1-4` doc comment), the implementation should record whether treating `infrastructure/metrics` as an allowed infra→infra dependency is an intentional, documented exception, or whether the metric should instead be set from the `buzz` adapter layer via `ConnectedRelayCount()`.
- A test verifies the gauge value changes on relay connect and disconnect.

#### FR-005: `BuzzConfig.NIP05` field is declared and YAML-decoded but never read anywhere

**File:** `internal/config/gateway_config.go:49`

`BuzzConfig.NIP05` (YAML tag `nip05`) is accepted from user configuration and documented in `support_docs/buzz-guide.md:65-72` as a configuration field, but a repo-wide search shows it is never read outside its own declaration — not by `cmd/nuimanbot/main.go`, not by `internal/adapter/gateway/buzz/gateway.go`, not by anything in `internal/infrastructure/nostr/`. A user who sets `nip05` in their config gets silent no-op behavior, which is worse than the field not existing at all, since it implies functionality that isn't there.

**Acceptance criteria:**
- Either: the `nip05` field is wired into whatever it was intended for (e.g., included in the `kind:10100` agent-profile event content, per NIP-05 identity-verification convention), with a test confirming it appears in the published profile event; or, if genuinely out of scope for this phase, the field and its documentation are removed (or explicitly marked reserved/no-effect, matching the existing precedent already used for `DMPolicy`) so it doesn't silently mislead operators.

#### FR-006: RBAC-denial and rate-limit-exceeded metrics are declared but never incremented anywhere

**File:** `internal/infrastructure/metrics/prometheus.go:218-241` (`RateLimitExceeded`, `SecurityValidationFailures`, `AuditEventsTotal` declarations); `internal/usecase/tool/service.go:119-224` (`ExecuteWithUser`, `auditPermissionDenial`)

This branch's cross-cutting RBAC fix means `ExecuteWithUser()` now enforces role and rate-limit checks for every platform, and denials are correctly captured in the durable file-based audit log via `securitySvc.Audit(...)` (`auditPermissionDenial`, `service.go:210-224`, and the rate-limit branch at `service.go:188-203`) — this part works. However, three Prometheus metrics that exist specifically for this purpose (`AuditEventsTotal{action,outcome}`, `RateLimitExceeded{user_id,action}`, `SecurityValidationFailures{reason}`) are never incremented anywhere in the codebase — confirmed by grep, each appears only at its own `promauto` declaration. This means RBAC denials and rate-limit rejections are auditable after the fact (by reading/parsing the audit log) but are invisible to real-time alerting or dashboards. This gap pre-dates this branch and is not Buzz-specific, but this branch is what makes RBAC enforcement universal across all platforms for the first time (FR-011/FR-012 in the spec), which makes the absence of a live alertable signal for "spike in RBAC denials" newly significant.

**Acceptance criteria:**
- `securitySvc.Audit(...)` (or the call sites in `auditPermissionDenial` / the rate-limit branch in `ExecuteWithUser`) increments `AuditEventsTotal.WithLabelValues(event.Action, event.Outcome)` and/or the more specific `RateLimitExceeded`/`SecurityValidationFailures` counters as appropriate.
- A test confirms that a simulated RBAC denial and a simulated rate-limit rejection each increment the corresponding metric.

#### FR-007: User documentation describes a non-functional environment-variable override for enabling Buzz

**File:** `support_docs/buzz-guide.md:50-54`

The guide states: *"Or via environment variables, if you prefer not to edit the config file: `export NUIMANBOT_GATEWAYS_BUZZ_ENABLED=true`."* This does not work. `internal/config/loader.go` has no `viper.AutomaticEnv()`/`SetEnvKeyReplacer`/`BindEnv` call anywhere; `applyEnvOverrides()` (`loader.go:225-291`) is a hand-coded whitelist of specific env vars that includes `NUIMANBOT_GATEWAYS_CLI_DEBUGMODE` but nothing for any `gateways.buzz.*` key. (The only Buzz-related env handling, `loader.go:187-189`, checks `v.IsSet("gateways.buzz.private_key")` against viper's config-file-derived state — it does not read an env var.) An operator following this doc verbatim would set the env var, restart, observe Buzz still disabled, and have no diagnostic explaining why.

**Acceptance criteria:**
- Either: `applyEnvOverrides()` is extended to actually support `NUIMANBOT_GATEWAYS_BUZZ_ENABLED` (and ideally the other Buzz config keys, for consistency with how `enabled` is documented), with a test confirming the env var takes effect; or, the "Or via environment variables" paragraph is removed from `support_docs/buzz-guide.md` and replaced with a note that Buzz configuration currently requires editing `config.yaml` directly (matching what other config fields in the same doc already imply).
- This is a documentation-only review; the fix itself belongs to a later remediation phase, but is called out with P1 severity because an operator acting on this doc today gets a silently broken result.

---

### P2 — Medium priority, polish/minor gaps

#### FR-008: `Send()`/`Start()`/`Stop()`/`handleEvents()` access shared fields without synchronization

**File:** `internal/adapter/gateway/buzz/gateway.go:46-59`

`Gateway.client`, `Gateway.cancel`, and `Gateway.messageHandler` are plain struct fields written in `Start()`/`OnMessage()` and read from `Send()`/`Stop()`/`handleEvents()` with no mutex protection, unlike `seen` which is correctly guarded by `seenMu`. This is benign under the tested call order (`OnMessage` → `Start` → later `Send`/`Stop`), and `go test -race` doesn't catch it because no existing test drives concurrent access — but it is a latent data race if a caller ever invokes `Send()` concurrently with `Start()`.

**Acceptance criteria:** Fields that can be written and read from different goroutines are protected by a mutex (or otherwise made safe, e.g. via `atomic.Pointer`), and a `-race`-run concurrency test exercises `Send()`/`Stop()` called concurrently with `Start()`.

#### FR-009: `agent_cache` and `loop_guard` maps grow unbounded for process lifetime

**Files:** `internal/adapter/gateway/buzz/agent_cache.go`, `internal/adapter/gateway/buzz/loop_guard.go`

Neither the `agentCache`'s `isAgent` map nor `loopGuard`'s `channels` map has TTL-based eviction. `loopGuard` is bounded in practice by the small configured channel set, but `agentCache` is keyed by every distinct sender pubkey ever observed — on a long-running process connected to a busy or public relay, this is unbounded memory growth over time.

**Acceptance criteria:** `agentCache` has a bounded size or TTL-based eviction policy for stale pubkey entries, with a test confirming old entries are evicted after the configured window/capacity is exceeded.

#### FR-010: `nostr.Filter.Since` is defined and marshaled but never populated

**File:** `internal/infrastructure/nostr/subscription.go:57-61`

`Filter.Since` exists in the struct and is included in `MarshalJSON`, but no `New*Filter` constructor or gateway code ever sets it. As a result, reconnects re-issue the exact same `REQ` frame built once at `Start()`, meaning there's no time-bounded resume/backfill of messages missed during a disconnect. This is plausibly an intentional Phase 1 scope cut (research.md references suggest as much), but as shipped, the field exists without being functional, which can read as "wired" when it isn't.

**Acceptance criteria:** Either populate `Since` on reconnect (based on the timestamp of the last successfully processed event) with a test confirming missed-message backfill after a simulated disconnect, or explicitly document `Since` as reserved/unused for this phase (matching the precedent set for `DMPolicy`).

#### FR-011: `Gateway` depends on the concrete `*user.Service` type rather than a small consumer-defined interface

**File:** `internal/adapter/gateway/buzz/gateway.go:48,64`

Not a Clean Architecture layering violation (adapter→usecase is a valid inward dependency), but inconsistent with this same PR's own pattern: `usecase/chat/service.go:44` correctly defines `UserService` as a small consumer-side interface per the project's stated interface-segregation convention (AGENTS.md), while `buzz.Gateway` takes the full concrete `*user.Service` struct.

**Acceptance criteria:** `buzz.Gateway` defines its own minimal interface for the subset of `user.Service` methods it actually calls (e.g., `GetUserByPlatformUID`/`CreateUser`), consistent with `chat.UserService`.

#### FR-012: Missing regression tests for kind:9000/10100 forgery and retry/error branches

**Files:** `internal/adapter/gateway/buzz/gateway_test.go`; `internal/adapter/gateway/buzz/gateway.go` (`publishAgentProfileBestEffort`, `resolveUser`)

Signature verification is provably safe today by code structure (single choke point, see the "no findings" section above), but there is no test that forges a kind:9000 or kind:10100 event and asserts `agentCache` remains unaffected — existing forgery tests (`TestGateway_ProcessEvent_UnsignedEvent_DroppedAndMetricIncremented`, `TestGateway_ProcessEvent_ForgedEvent_DroppedAndMetricIncremented`) only exercise kind:9. A future refactor that adds a second dispatch path wouldn't be caught by the current suite. Separately, `publishAgentProfileBestEffort` (40% coverage) and the `errors.Is(err, domain.ErrConflict)`/lookup-failure branches of `resolveUser` (66–75% coverage) are exercised only on the happy path.

**Acceptance criteria:** A test forges kind:9000 and kind:10100 events with invalid signatures and asserts `agentCache` state is unchanged after `processEvent` handles them. Tests cover `publishAgentProfileBestEffort`'s retry-exhaustion path and `resolveUser`'s conflict/lookup-error branches.

#### FR-013: `buzz_events_received_total` metric name/help text is broader than its actual scope

**File:** `internal/infrastructure/metrics/prometheus.go:252-258`; increment site `internal/adapter/gateway/buzz/gateway.go:269`

The metric's help text describes "verified Buzz events received" generically, but it is only incremented in `processChannelMessage`, i.e. only for kind:9 channel messages — not for kind:9000/kind:10100 events, which also flow through the same verified pipeline (`processEvent`). Not a correctness bug (the metric does accurately count what it counts), but the name invites an operator to assume it covers all Buzz event traffic.

**Acceptance criteria:** Either rename/re-scope the metric's help text to clarify it counts channel messages specifically, or add a label/dimension (or companion counter) that also captures kind:9000/10100 event volume.

#### FR-014: Pre-existing stale code example in `documentation/technical-details.md` adjacent to the new Buzz content

**File:** `documentation/technical-details.md:175-182`

This predates the Buzz branch and is not itself a claim made about Buzz, but it sits in a file this branch modified and is adjacent to the (accurate) new Buzz role table: the doc shows a `SkillPermissions` map with `calculator`/`datetime` mapped to `RoleUser`, while the actual code (`internal/usecase/tool/permissions.go:12`, variable now named `ToolPermissions`) maps them to `RoleGuest`. Worth fixing in the same documentation pass to avoid confusing readers who compare it against the new, correct Buzz Guest-role table nearby.

**Acceptance criteria:** The stale example is updated to reflect the current `ToolPermissions` variable name and role assignments.

#### FR-015 (informational — no fix required, flag for future review): dormant subagent tool-executor path

**File:** `internal/usecase/tool/service.go` (context: `subagent.ToolExecutor.Execute`, `subagent.NewSubagentExecutor`)

`subagent.NewSubagentExecutor` and its `ToolExecutor.Execute` are not called anywhere in `cmd/nuimanbot/main.go` today — confirmed dormant, not a live RBAC bypass. No action needed now, but if this is ever wired up to call `tool.Service` directly, it must go through `ExecuteWithUser()`, not the unchecked `Execute()`, to preserve the RBAC guarantees this branch establishes everywhere else.

**Acceptance criteria:** N/A for this branch. Recommend a code comment or tracked follow-up item noting the RBAC requirement for whoever wires this path up in the future.

#### FR-016: Buzz gateway performs its own redundant user resolution alongside `ChatService`'s

**File:** `internal/adapter/gateway/buzz/gateway.go:271-275,356-368`

`Gateway` calls `resolveUser`/`CreateUser` itself, in addition to `ChatService.resolveUser` now doing the same for every platform (as of this branch's RBAC work). This is self-documented in the code as a deliberate, harmless, idempotent double-lookup kept to avoid touching already-merged Phase 1 code. Functionally harmless, but Buzz is the only gateway carrying this extra usecase-layer dependency, which is a minor architectural asymmetry worth cleaning up opportunistically.

**Acceptance criteria:** Either remove the Buzz-gateway-level user resolution now that `ChatService` performs it uniformly, or document explicitly (beyond the inline comment) why Buzz needs it independently and the other gateways don't.

---

## Summary Table

| # | Finding | Priority |
|---|---|---|
| FR-001 | `Send()` nil-pointer panic risk on `g.client` | P1 |
| FR-002 | `Stop()` has 0% test coverage | P1 |
| FR-003 | `Start()` error/guard paths untested | P1 |
| FR-004 | `buzz_relay_connections` gauge declared but never set | P1 |
| FR-005 | `BuzzConfig.NIP05` field declared but never read | P1 |
| FR-006 | RBAC-denial/rate-limit metrics declared but never incremented | P1 |
| FR-007 | Doc describes non-functional `NUIMANBOT_GATEWAYS_BUZZ_ENABLED` env var | P1 |
| FR-008 | Unsynchronized shared fields in `Gateway` (latent data race) | P2 |
| FR-009 | Unbounded `agent_cache`/`loop_guard` map growth | P2 |
| FR-010 | `nostr.Filter.Since` defined but never populated | P2 |
| FR-011 | `Gateway` depends on concrete `*user.Service` vs. own interface | P2 |
| FR-012 | Missing kind:9000/10100 forgery tests + retry/error-branch tests | P2 |
| FR-013 | `buzz_events_received_total` scope narrower than name implies | P2 |
| FR-014 | Stale `SkillPermissions`/`ToolPermissions` doc example (pre-existing) | P2 |
| FR-015 | Dormant subagent executor path (informational) | P2 |
| FR-016 | Redundant Buzz-gateway user resolution | P2 |

**Total: 0 P0, 7 P1, 9 P2 (16 findings)**
