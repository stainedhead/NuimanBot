# Spec: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Source:** Code and Design Review of `feat/nuimanbot-support-buzz` (see References)

## Executive Summary

This spec covers remediation of the 16 findings from the code/design review of the completed Buzz (Nostr-based multi-agent chat) gateway feature. The review found **no P0 blockers, no security holes, and no broken build** — Clean Architecture boundaries hold, signature verification is sound across all consumed event kinds, and RBAC bypass paths are closed. The findings that exist cluster into four areas: (1) a nil-pointer panic risk and untested lifecycle methods in `Gateway.Send()`/`Start()`/`Stop()`; (2) two dead/unwired declarations (`buzz_relay_connections` metric, `NIP05` config field); (3) a pre-existing, non-Buzz-specific observability gap (RBAC-denial/rate-limit metrics never incremented) that is explicitly deferred; (4) one misleading line of user-facing documentation about a non-functional env var override.

This pass fixes 14 of the 16 findings. **FR-006 is deferred** (pre-existing, not Buzz-specific — tracked as a separate observability-hardening follow-up). **FR-015 is informational only** — no fix required, just a tracked follow-up note.

## Problem Statement

The Buzz gateway branch (`feat/nuimanbot-support-buzz`) is functionally complete and mergeable, but a structured code/design review surfaced 16 findings spanning reliability (untested lifecycle paths, a panic risk), observability (an unwired metric, an unbacked reconnect-resume mechanism), configuration honesty (a no-op config field), documentation accuracy (a broken env-var instruction, a stale code example), and minor architectural consistency (interface segregation, redundant user resolution). Left unaddressed, these degrade operational confidence in production (silent panics, invisible relay-connection state, silently dropped messages during relay outages) and mislead operators following the shipped docs.

## Goals / Non-Goals

**Goals:**
- Close the P1 reliability/observability gaps in `Gateway.Send()`/`Start()`/`Stop()` and the `buzz_relay_connections` metric.
- Make the `NIP05` config field and the documented env-var override honest (either functional or clearly marked as not).
- Implement reconnect backfill via `Filter.Since` so relay outages don't silently drop messages.
- Clean up the P2 polish items (data race, unbounded cache growth, interface segregation, test coverage gaps, metric naming, stale docs, redundant user resolution).

**Non-Goals:**
- **FR-006** (RBAC-denial/rate-limit metrics never incremented) is explicitly **out of scope** for this pass — it is a pre-existing, cross-platform gap not introduced by Buzz. Tracked separately.
- **FR-015** (dormant subagent tool-executor path) requires no code change in this pass — informational tracking only.
- No new features, no re-litigation of already-resolved design questions (see Open Questions below — all resolved by the product owner prior to this spec).

## User Requirements

Functional requirements below are derived directly from the review PRD's findings (`FR-XXX` numbering carried over verbatim from the review). Each already carries its own acceptance criteria from the review; they are not re-derived here.

### FR-001: `Gateway.Send()` has no nil-guard on `g.client`
**Priority:** P1 | **Cluster:** A
**File:** `internal/adapter/gateway/buzz/gateway.go:176-193`

`Send()` calls `g.client.Publish(...)` with no check that `g.client` is non-nil, risking a nil-pointer panic if invoked before `Start()` completes or after `Stop()`/a failed `Start()`. Regression relative to the established Slack-gateway guard pattern (`internal/adapter/gateway/slack/gateway.go:84-86`).

**Acceptance criteria:**
- `Gateway.Send()` returns a descriptive error (not a panic) when called with a nil/uninitialized `g.client`.
- A test exists that calls `Send()` on a `Gateway` that was constructed but never `Start()`ed (or was `Stop()`ped), asserting a graceful error return rather than a panic.

### FR-002: `Gateway.Stop()` has 0% test coverage
**Priority:** P1 | **Cluster:** A
**File:** `internal/adapter/gateway/buzz/gateway.go:162`

No test calls `gw.Stop()`. Connection-lifecycle shutdown is completely unverified.

**Acceptance criteria:**
- At least one test calls `Stop()` on a started `Gateway` and asserts the expected post-stop state (context cancelled, no further events delivered, resources released, idempotent on double-`Stop()` if supported).

### FR-003: `Gateway.Start()` error/guard paths are untested
**Priority:** P1 | **Cluster:** A
**File:** `internal/adapter/gateway/buzz/gateway.go:82-100`

Three error paths have no test: missing-relay guard (line 84), missing-private-key guard (line 88), `nostr.Client.Start` failure wrap (line 100).

**Acceptance criteria:**
- A test exists for each of the three error paths, asserting `Start()` returns the expected error and does not leave the gateway in a partially-initialized state a subsequent `Send()`/`Stop()` could panic on (ties to FR-001).

### FR-004: `buzz_relay_connections` Prometheus gauge declared but never set
**Priority:** P1 | **Cluster:** B
**Files:** `internal/infrastructure/metrics/prometheus.go:244-250` (declaration); `internal/adapter/gateway/buzz/gateway.go` (missing wiring)

`BuzzRelayConnections` is a `GaugeVec` with zero `.Set()`/`.Inc()`/`.Dec()`/`.WithLabelValues()` calls anywhere. `ConnectedRelayCount()` (`internal/infrastructure/nostr/client.go:214-218`) exists but has no production caller — only tests use it.

**Decided (product owner, 2026-08-02):** the metric is set from the `buzz` adapter layer via `ConnectedRelayCount()`, not from `internal/infrastructure/nostr` directly. This keeps `nostr/` protocol-only with zero Prometheus awareness, matching its own stated goal of staying a swappable, metrics-agnostic protocol layer.

**Acceptance criteria:**
- `buzz.Gateway` calls `g.client.ConnectedRelayCount()` (or an equivalent per-relay status source) and sets `BuzzRelayConnections.WithLabelValues(relayURL, status).Set(...)` from the adapter layer — e.g. via a small ticker, or reacting to connect/disconnect callbacks — not from `internal/infrastructure/nostr`.
- A test verifies the gauge value changes on relay connect and disconnect.

### FR-005: `BuzzConfig.NIP05` field declared and YAML-decoded but never read
**Priority:** P1 | **Cluster:** C
**File:** `internal/config/gateway_config.go:49`

Accepted from user config and documented (`support_docs/buzz-guide.md:65-72`), but never read outside its own declaration. Silent no-op — worse than not existing since it implies functionality that isn't there.

**Decided (product owner, 2026-08-02):** mark reserved now, matching the `DMPolicy` precedent. NIP-05 verification was explicitly called out as optional/deferred in the original PRD (§6.5).

**Acceptance criteria:**
- `BuzzConfig.NIP05`'s doc comment is updated to state it's reserved for a future phase and not currently read (referencing PRD §6.5), matching `DMPolicy`'s existing treatment.
- `support_docs/buzz-guide.md:65-72` is corrected to describe `nip05` as reserved/no-effect rather than implying it's functional.

### FR-006 [DEFERRED — out of scope for this spec]: RBAC-denial and rate-limit metrics declared but never incremented
**Priority:** P1 (deferred) | **Cluster:** excluded

`AuditEventsTotal`, `RateLimitExceeded`, `SecurityValidationFailures` are declared but never incremented anywhere (confirmed by grep — each appears only at its own `promauto` declaration). Denials are correctly captured in the durable audit log; this gap is about real-time alerting/dashboards specifically.

**Decided (product owner, 2026-08-02): deferred, out of scope for this remediation pass.** Pre-existing, not Buzz-specific. Tracked as a separate observability-hardening item.

**Not tasked in this spec.** Acceptance criteria retained for the future follow-up item only:
- `securitySvc.Audit(...)` call sites increment `AuditEventsTotal.WithLabelValues(...)` and/or `RateLimitExceeded`/`SecurityValidationFailures` as appropriate.
- A test confirms a simulated RBAC denial and rate-limit rejection each increment the corresponding metric.

### FR-007: User documentation describes a non-functional environment-variable override
**Priority:** P1 | **Cluster:** C
**File:** `support_docs/buzz-guide.md:50-54`

The guide claims `export NUIMANBOT_GATEWAYS_BUZZ_ENABLED=true` works. It doesn't — `internal/config/loader.go` has no `viper.AutomaticEnv()`/`SetEnvKeyReplacer`/`BindEnv`; `applyEnvOverrides()` (`loader.go:225-291`) is a hand-coded whitelist with nothing for `gateways.buzz.*`.

**Acceptance criteria:**
- Either: extend `applyEnvOverrides()` to actually support `NUIMANBOT_GATEWAYS_BUZZ_ENABLED` (and ideally other Buzz config keys), with a test confirming the env var takes effect; or, remove the "Or via environment variables" paragraph from `support_docs/buzz-guide.md` and replace with a note that Buzz configuration currently requires editing `config.yaml` directly.
- Implementation note: the review PRD flags this as "documentation-only review, fix belongs to a later remediation phase" but it is included as a task in this spec since this spec IS that later remediation phase.

### FR-008: `Send()`/`Start()`/`Stop()`/`handleEvents()` access shared fields without synchronization
**Priority:** P2 | **Cluster:** A
**File:** `internal/adapter/gateway/buzz/gateway.go:46-59`

`Gateway.client`, `Gateway.cancel`, `Gateway.messageHandler` are plain struct fields written in `Start()`/`OnMessage()` and read from `Send()`/`Stop()`/`handleEvents()` with no mutex protection (unlike `seen`, guarded by `seenMu`). Benign under tested call order; latent data race if `Send()` is ever called concurrently with `Start()`.

**Acceptance criteria:**
- Fields that can be written and read from different goroutines are protected by a mutex (or otherwise made safe, e.g. `atomic.Pointer`).
- A `-race`-run concurrency test exercises `Send()`/`Stop()` called concurrently with `Start()`.

### FR-009: `agent_cache` and `loop_guard` maps grow unbounded for process lifetime
**Priority:** P2 | **Cluster:** D
**Files:** `internal/adapter/gateway/buzz/agent_cache.go`, `internal/adapter/gateway/buzz/loop_guard.go`

Neither `agentCache.isAgent` nor `loopGuard.channels` has TTL-based eviction. `loopGuard` is bounded in practice by the small configured channel set; `agentCache` is keyed by every distinct sender pubkey ever observed — unbounded growth on a long-running process connected to a busy/public relay.

**Decided (product owner, 2026-08-02):** left to the implementer, consistent with how Phase 2's loop-prevention heuristic was handled — pick a reasonable TTL/capacity bound, document the choice and rationale in `implementation-notes.md`, cover with a test. No existing cache-eviction pattern in this codebase to match.

**Acceptance criteria:**
- `agentCache` has a bounded size or TTL-based eviction policy for stale pubkey entries, with a test confirming old entries are evicted after the configured window/capacity is exceeded.

### FR-010: `nostr.Filter.Since` is defined and marshaled but never populated
**Priority:** P1 (reclassified from P2) | **Cluster:** D
**File:** `internal/infrastructure/nostr/subscription.go:57-61`

`Filter.Since` exists and is included in `MarshalJSON`, but no `New*Filter` constructor or gateway code ever sets it. Reconnects re-issue the exact same `REQ` frame built once at `Start()` — no time-bounded resume/backfill of messages missed during a disconnect.

**Decided (product owner, 2026-08-02): implement reconnect backfill.** This was NOT a confirmed prior Phase 1 scope cut — the review's original guess doesn't hold up (`specs/260802-nuimanbot-support-buzz/research.md` and `spec.md` contain no mention of `Since`/backfill anywhere). Spec.md §6.10's reliability NFR requires the gateway to "continue operating" through partial relay outages — silently missing messages during a reconnect window undercuts that guarantee. Reclassified P2→P1.

**Acceptance criteria:**
- Populate `Since` on reconnect, based on the timestamp of the last successfully processed event, so a relay that drops and reconnects catches up on events missed during the outage.
- Test: simulate a disconnect, publish N events to the relay while it's down, reconnect, and assert all N are eventually delivered (not silently lost).

### FR-011: `Gateway` depends on the concrete `*user.Service` type rather than a small consumer-defined interface
**Priority:** P2 | **Cluster:** A
**File:** `internal/adapter/gateway/buzz/gateway.go:48,64`

Not a layering violation, but inconsistent with this same branch's own pattern: `usecase/chat/service.go:44` defines `UserService` as a small consumer-side interface per AGENTS.md's interface-segregation convention, while `buzz.Gateway` takes the full concrete `*user.Service` struct.

**Acceptance criteria:**
- `buzz.Gateway` defines its own minimal interface for the subset of `user.Service` methods it actually calls (e.g., `GetUserByPlatformUID`/`CreateUser`), consistent with `chat.UserService`.

### FR-012: Missing regression tests for kind:9000/10100 forgery and retry/error branches
**Priority:** P2 | **Cluster:** A
**Files:** `internal/adapter/gateway/buzz/gateway_test.go`; `internal/adapter/gateway/buzz/gateway.go` (`publishAgentProfileBestEffort`, `resolveUser`)

Existing forgery tests only exercise kind:9. No test forges kind:9000/10100 and asserts `agentCache` remains unaffected. `publishAgentProfileBestEffort` (40% coverage) and the `errors.Is(err, domain.ErrConflict)`/lookup-failure branches of `resolveUser` (66-75% coverage) are exercised only on the happy path.

**Acceptance criteria:**
- A test forges kind:9000 and kind:10100 events with invalid signatures and asserts `agentCache` state is unchanged after `processEvent` handles them.
- Tests cover `publishAgentProfileBestEffort`'s retry-exhaustion path and `resolveUser`'s conflict/lookup-error branches.

### FR-013: `buzz_events_received_total` metric name/help text is broader than its actual scope
**Priority:** P2 | **Cluster:** A
**File:** `internal/infrastructure/metrics/prometheus.go:252-258`; increment site `internal/adapter/gateway/buzz/gateway.go:269`

Help text describes "verified Buzz events received" generically, but it's only incremented in `processChannelMessage` — i.e. only kind:9 channel messages, not kind:9000/10100.

**Acceptance criteria:**
- Either rename/re-scope the metric's help text to clarify it counts channel messages specifically, or add a label/dimension (or companion counter) that also captures kind:9000/10100 event volume.

### FR-014: Pre-existing stale code example in `documentation/technical-details.md`
**Priority:** P2 | **Cluster:** C
**File:** `documentation/technical-details.md:175-182`

Predates the Buzz branch; sits adjacent to the new (accurate) Buzz role table. Shows a `SkillPermissions` map with `calculator`/`datetime` mapped to `RoleUser`, while actual code (`internal/usecase/tool/permissions.go:12`, now named `ToolPermissions`) maps them to `RoleGuest`.

**Acceptance criteria:**
- The stale example is updated to reflect the current `ToolPermissions` variable name and role assignments.

### FR-015 [INFORMATIONAL ONLY — no fix task]: dormant subagent tool-executor path
**Priority:** P2 (informational) | **Cluster:** none (tracked, not tasked)
**File:** `internal/usecase/tool/service.go` (context: `subagent.ToolExecutor.Execute`, `subagent.NewSubagentExecutor`)

`subagent.NewSubagentExecutor`/`ToolExecutor.Execute` are not called anywhere in `cmd/nuimanbot/main.go` today — confirmed dormant, not a live RBAC bypass.

**Acceptance criteria:** N/A for this spec. Recommend a code comment or tracked follow-up item noting that if this path is ever wired up, it must go through `ExecuteWithUser()`, not the unchecked `Execute()`.

### FR-016: Buzz gateway performs its own redundant user resolution alongside `ChatService`'s
**Priority:** P2 | **Cluster:** A
**File:** `internal/adapter/gateway/buzz/gateway.go:271-275,356-368`

`Gateway` calls `resolveUser`/`CreateUser` itself, in addition to `ChatService.resolveUser` now doing the same for every platform (as of this branch's RBAC work). Self-documented as a deliberate, harmless, idempotent double-lookup kept to avoid touching already-merged Phase 1 code.

**Acceptance criteria:**
- Either remove the Buzz-gateway-level user resolution now that `ChatService` performs it uniformly, or document explicitly (beyond the inline comment) why Buzz needs it independently and the other gateways don't.

## Non-Functional Requirements

- **TDD, no exceptions.** Red (failing test capturing acceptance criteria) → Green (minimal fix) → Refactor (mandatory) for every FR in this spec, per AGENTS.md.
- **Concurrency safety.** FR-008's fix must be verified with `go test -race`.
- **Reliability.** FR-010's backfill must not duplicate already-delivered events; dedupe (`seen`/`seenMu`) already exists and should suffice given `Since` is inclusive of a slightly-earlier boundary.
- **Quality gates.** Every cluster must pass the full gate (`go fmt`, `go vet`, `golangci-lint run`, `go test ./...`, `go build -o bin/nuimanbot ./cmd/nuimanbot`) before merge, per AGENTS.md.

## System Architecture

**Affected layers:**
- `internal/adapter/gateway/buzz/` — primary surface (Cluster A: gateway.go lifecycle/structure; Cluster B: metric wiring; Cluster D: agent_cache.go)
- `internal/infrastructure/nostr/` — `subscription.go` (Since/backfill, Cluster D)
- `internal/infrastructure/metrics/prometheus.go` — no structural change; consumed by Cluster B/FR-013
- `internal/config/gateway_config.go` — `NIP05` doc comment (Cluster C)
- `support_docs/buzz-guide.md`, `documentation/technical-details.md` — doc-only changes (Cluster C)

No new components. No changes to domain or usecase layers required by this spec (FR-011's new interface lives in the adapter layer, consumer-side, consistent with existing `chat.UserService` pattern).

## Scope of Changes

**Files to modify:**
- `internal/adapter/gateway/buzz/gateway.go` (FR-001, 002, 003, 004, 008, 011, 012, 013, 016)
- `internal/adapter/gateway/buzz/gateway_test.go` (FR-001, 002, 003, 004, 008, 012)
- `internal/adapter/gateway/buzz/agent_cache.go` (FR-009)
- `internal/adapter/gateway/buzz/agent_cache_test.go` (FR-009)
- `internal/infrastructure/nostr/subscription.go` (FR-010)
- `internal/infrastructure/nostr/subscription_test.go` / `client_test.go` (FR-010)
- `internal/config/gateway_config.go` (FR-005)
- `support_docs/buzz-guide.md` (FR-005, FR-007)
- `documentation/technical-details.md` (FR-014)

**No new dependencies.**

## Breaking Changes

None. All changes are internal reliability/observability/documentation fixes; no public API, config schema, or CLI surface changes (the `NIP05` field remains in the config schema, just documented as reserved/no-op).

## Success Criteria and Acceptance Criteria

- All 14 tasked FRs (FR-001–005, FR-007–014, FR-016) pass their individual acceptance criteria as specified above.
- Full quality gate passes after each cluster merges: `go fmt ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...` (including `-race` for FR-008), `go build -o bin/nuimanbot ./cmd/nuimanbot`.
- No regression in any of the "Dimensions Reviewed With No Findings" areas from the original review (signature verification, RBAC bypass paths, CLI RoleAdmin spoofing resistance, key leakage, input validation, Clean Architecture layering).
- FR-006 remains explicitly untouched/deferred; FR-015 has a tracked follow-up note but no code change.

## Risks and Mitigation

- **Risk:** Cluster A touches a single large file (`gateway.go`) across 8 findings — merge conflict risk if parallelized carelessly. **Mitigation:** single owner, sequential work within Cluster A, per the review PRD's own guidance.
- **Risk:** FR-004 and Cluster A both touch `gateway.go`. **Mitigation:** coordinate FR-004 with Cluster A's owner, or fold FR-004 into Cluster A's sequence (spec allows either).
- **Risk:** FR-010's backfill could reintroduce duplicate-delivery bugs. **Mitigation:** rely on existing `seen`/`seenMu` dedupe; add explicit test asserting no duplicates across a reconnect-with-backfill cycle.
- **Risk:** FR-009's TTL/capacity choice is a fresh design call with no existing precedent. **Mitigation:** document rationale in `implementation-notes.md` as directed by the product owner's decision.

## Timeline and Milestones

Per the review PRD's cluster breakdown — see `tasks.md` for the full breakdown:
- **Cluster A** (gateway.go lifecycle, FR-001/002/003/008/011/012/013/016): sequential, single owner, do first (P1 items within it before P2 items).
- **Cluster B** (FR-004, observability wiring): parallel with A, coordinate on `gateway.go` overlap.
- **Cluster C** (FR-005/007/014, config+docs): parallel with A/B.
- **Cluster D** (FR-009/010, cache+subscription): parallel with A/B/C.

Each cluster merges back to the integration branch with a full quality-gate re-run before the next cluster starts, per AGENTS.md's TDD/quality-gate workflow.

## References

- Source review PRD: `specs/260802-nuimanbot-support-buzz-auto-review/nuimanbot-support-buzz-auto-review-PRD.md`
- Original feature spec: `specs/260802-nuimanbot-support-buzz/spec.md`
- `AGENTS.md` (repo root) — TDD, quality gates, Clean Architecture rules
