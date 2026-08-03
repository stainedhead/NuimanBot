# Tasks: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Status:** Planning

## Progress Summary

14/14 tasks complete (14 tasked FRs; FR-006 deferred/excluded, FR-015 informational/no task). Clusters A (A.1-A.8), B (B.1), C (C.1-C.3), D (D.1-D.2) all complete as of 2026-08-02 — see status.md for per-cluster activity log and implementation-notes.md for design decisions. Remaining: I.1 cross-cluster integration verification.

## Scope Notes

- **FR-006 is excluded from this task list** — deferred per product-owner decision (2026-08-02), out of scope for this remediation pass. Tracked separately as a future observability-hardening item. No task below implements it.
- **FR-015 is informational only** — no implementation task. Recommended action: add a code comment near `subagent.NewSubagentExecutor`/`ToolExecutor.Execute` (`internal/usecase/tool/service.go`) noting that any future wiring must go through `ExecuteWithUser()`, not `Execute()`. This is tracked here as an optional documentation touch, not a Red-Green-Refactor task, since there is no behavior to test.
- Every task below follows **mandatory Red-Green-Refactor** per AGENTS.md: write the failing test first (Red), implement the minimal fix (Green), then refactor with tests kept green (Refactor) — the refactor step is not optional.
- Fix order within and across clusters: **P1 before P2.**

---

## Cluster A — `gateway.go` Lifecycle & Structure

**Owner:** single agent/worktree, sequential (shared-file cluster — do not parallelize within this cluster)
**Files:** `internal/adapter/gateway/buzz/gateway.go`, `internal/adapter/gateway/buzz/gateway_test.go`

### A.1 — FR-001: Nil-guard on `Gateway.Send()` [P1]
- **Dependencies:** none (do first — establishes the guard pattern other lifecycle tests rely on)
- **Duration estimate:** S
- **Red:** Test calling `Send()` on a constructed-but-never-`Start()`ed `Gateway`, asserting a returned error (not a panic).
- **Green:** Add `if g.client == nil { return fmt.Errorf(...) }` guard in `Send()`, matching the Slack gateway pattern (`internal/adapter/gateway/slack/gateway.go:84-86`).
- **Refactor:** Confirm error message is descriptive and consistent in tone with other gateway error messages in this codebase.
- **Acceptance criteria:** See spec.md FR-001.

### A.2 — FR-002: Test coverage for `Gateway.Stop()` [P1]
- **Dependencies:** A.1 (reuses the same started/not-started `Gateway` test fixtures)
- **Duration estimate:** S
- **Red:** Test that starts a `Gateway`, calls `Stop()`, and asserts context cancellation / no further event delivery / idempotency on double-`Stop()`.
- **Green:** Fix any gaps `Stop()` has revealed under test (if `Stop()` already behaves correctly, this may be test-only; otherwise implement missing cleanup).
- **Refactor:** Ensure `Stop()` composes cleanly with A.1's nil-guard (a `Send()` after `Stop()` should hit the FR-001 guard, not panic).
- **Acceptance criteria:** See spec.md FR-002.

### A.3 — FR-003: Test coverage for `Start()` error/guard paths [P1]
- **Dependencies:** A.1, A.2
- **Duration estimate:** S
- **Red:** Three tests: missing-relay guard, missing-private-key guard, `nostr.Client.Start` failure wrap — each asserting the specific expected error string/type.
- **Green:** Confirm/fix that each guard path leaves the `Gateway` in a state where a subsequent `Send()`/`Stop()` doesn't panic (ties to A.1).
- **Refactor:** Consolidate any duplicated test setup across A.1/A.2/A.3 into shared test helpers.
- **Acceptance criteria:** See spec.md FR-003.

*(Cluster A pauses here — P1 items A.1–A.3 complete. FR-004/005/007/010 in Clusters B/C/D are also P1 and should complete before Cluster A resumes P2 work, per AGENTS.md's P0→P1→P2 ordering across the whole spec. In practice, since clusters run in parallel worktrees, Cluster A's owner may proceed directly to A.4+ once A.1–A.3 are done, provided the overall spec's P1 items are tracked to completion in status.md before any cluster's P2 work is considered "final.")*

### A.4 — FR-008: Synchronize shared `Gateway` fields [P2]
- **Dependencies:** A.1, A.2, A.3
- **Duration estimate:** M
- **Red:** `-race`-run test that calls `Send()`/`Stop()` concurrently with `Start()`.
- **Green:** Add mutex protection (or `atomic.Pointer`) around `client`, `cancel`, `messageHandler`.
- **Refactor:** Confirm lock scope is minimal (no lock held across blocking I/O); compare against `seen`/`seenMu`'s existing pattern for consistency.
- **Acceptance criteria:** See spec.md FR-008.

### A.5 — FR-011: Consumer-side interface for user-service dependency [P2]
- **Dependencies:** none within Cluster A (independent of A.1–A.4, but sequenced here per file-ownership)
- **Duration estimate:** S
- **Red:** Existing tests should compile against a new narrow interface with a mock/fake implementation substituted for `*user.Service`.
- **Green:** Define `buzzUserService`-style interface in the `buzz` package (see data-dictionary.md), change `Gateway`'s field type, update constructor signature.
- **Refactor:** Verify no other adapter code still references the concrete `*user.Service` type unnecessarily.
- **Acceptance criteria:** See spec.md FR-011.

### A.6 — FR-012: Forgery + retry/error-branch test coverage [P2]
- **Dependencies:** none within Cluster A
- **Duration estimate:** M
- **Red:** Forged kind:9000/10100 events with invalid signatures, asserting `agentCache` unchanged; tests targeting `publishAgentProfileBestEffort`'s retry-exhaustion path and `resolveUser`'s conflict/lookup-error branches.
- **Green:** Fix any gaps these tests reveal (expected to be test-only additions, since the review confirmed signature verification is already structurally sound).
- **Refactor:** Clean up test fixtures/helpers for forged-event construction if reused across A.6 and existing forgery tests.
- **Acceptance criteria:** See spec.md FR-012.

### A.7 — FR-013: `buzz_events_received_total` scope clarification [P2]
- **Dependencies:** none within Cluster A
- **Duration estimate:** S
- **Red:** Test/assertion (or doc-comment-linting, if applicable) confirming metric help text matches actual increment scope.
- **Green:** Rename help text/labels, or add companion counter/label for kind:9000/10100 volume (implementer's choice, per spec.md FR-013).
- **Refactor:** Confirm consistency with FR-004's metric-naming conventions.
- **Acceptance criteria:** See spec.md FR-013.

### A.8 — FR-016: Remove or document redundant user resolution [P2]
- **Dependencies:** A.5 (interface change may affect this code path)
- **Duration estimate:** S
- **Red:** Test asserting behavior is unchanged whether Buzz's own resolution is removed (if removal is chosen) — i.e., `ChatService`'s resolution alone is sufficient.
- **Green:** Either delete Buzz's own `resolveUser`/`CreateUser` calls, or add explicit documentation (beyond the inline comment) justifying why Buzz needs it independently.
- **Refactor:** If removed, confirm no dead helper code remains.
- **Acceptance criteria:** See spec.md FR-016.

---

## Cluster B — Observability Wiring

**Owner:** parallel with A; coordinate on `gateway.go` overlap (see plan.md)
**Files:** `internal/adapter/gateway/buzz/gateway.go`, `internal/adapter/gateway/buzz/gateway_test.go`, possibly `internal/infrastructure/nostr/client.go` (read-only use of existing `ConnectedRelayCount()`)

### B.1 — FR-004: Wire `buzz_relay_connections` gauge via `ConnectedRelayCount()` [P1]
- **Dependencies:** none (coordinate scheduling with Cluster A to avoid concurrent `gateway.go` edits)
- **Duration estimate:** M
- **Red:** Test asserting the gauge value changes on simulated relay connect and disconnect.
- **Green:** Adapter-layer code (ticker or callback-driven, per research.md question 3) calls `g.client.ConnectedRelayCount()` and sets `BuzzRelayConnections.WithLabelValues(relayURL, status).Set(...)`.
- **Refactor:** Confirm `internal/infrastructure/nostr/` remains free of Prometheus imports (per the resolved architectural decision).
- **Acceptance criteria:** See spec.md FR-004.

---

## Cluster C — Config & Docs

**Owner:** parallel with A/B
**Files:** `internal/config/gateway_config.go`, `support_docs/buzz-guide.md`, `documentation/technical-details.md`

### C.1 — FR-005: Mark `NIP05` config field reserved [P1]
- **Dependencies:** none (do before C.2, both touch `buzz-guide.md`)
- **Duration estimate:** S
- **Red:** Not applicable in the traditional sense (doc-comment/prose change) — if feasible, a lint/test asserting the doc comment contains "reserved"/"not currently read" language, else treat as a direct documentation edit with review as the verification step.
- **Green:** Update `BuzzConfig.NIP05` doc comment (referencing PRD §6.5) and `support_docs/buzz-guide.md:65-72`, matching `DMPolicy`'s existing treatment.
- **Refactor:** N/A (prose only).
- **Acceptance criteria:** See spec.md FR-005.

### C.2 — FR-007: Fix or remove non-functional env-var documentation [P1]
- **Dependencies:** C.1 (both touch `buzz-guide.md` — sequence to avoid conflicting edits)
- **Duration estimate:** S (doc-only path) or M (if extending `applyEnvOverrides()`)
- **Red:** If extending `applyEnvOverrides()`: test confirming `NUIMANBOT_GATEWAYS_BUZZ_ENABLED` takes effect. If doc-only: no test, direct prose fix.
- **Green:** Either implement the env var support in `internal/config/loader.go`'s `applyEnvOverrides()`, or remove/replace the misleading paragraph in `support_docs/buzz-guide.md`.
- **Refactor:** If implementing, confirm consistency with the existing `NUIMANBOT_GATEWAYS_CLI_DEBUGMODE` whitelist pattern.
- **Acceptance criteria:** See spec.md FR-007. **Decision needed at implementation time:** which of the two acceptance-criteria options (implement vs. document) — recommend implementing only if scope stays small (single bool), else document-only to avoid scope creep into a full `viper.AutomaticEnv()` migration.

### C.3 — FR-014: Fix stale `SkillPermissions`/`ToolPermissions` doc example [P2]
- **Dependencies:** none (independent file/section from C.1/C.2)
- **Duration estimate:** S
- **Red:** N/A (doc-only).
- **Green:** Update `documentation/technical-details.md:175-182` to reflect current `ToolPermissions` variable name and `RoleGuest` mapping.
- **Refactor:** N/A.
- **Acceptance criteria:** See spec.md FR-014.

---

## Cluster D — Cache & Subscription

**Owner:** parallel with A/B/C
**Files:** `internal/adapter/gateway/buzz/agent_cache.go`, `internal/adapter/gateway/buzz/agent_cache_test.go`, `internal/infrastructure/nostr/subscription.go`, `internal/infrastructure/nostr/subscription_test.go`/`client_test.go`

### D.1 — FR-009: Bounded/TTL eviction for `agentCache` [P2]
- **Dependencies:** none
- **Duration estimate:** M
- **Red:** Test confirming old entries are evicted after the configured window/capacity is exceeded.
- **Green:** Implement TTL or capacity bound (implementer's design choice — document rationale in implementation-notes.md, per the product owner's deferred decision).
- **Refactor:** Confirm `loopGuard`'s `channels` map genuinely doesn't need the same treatment (per spec.md FR-009's note that it's bounded in practice) rather than assuming.
- **Acceptance criteria:** See spec.md FR-009.

### D.2 — FR-010: Populate `Filter.Since` on reconnect (backfill) [P1]
- **Dependencies:** none within Cluster D (independent file from D.1); resolve research.md questions 1–2 (high-water-mark granularity, dedupe capacity) before implementing
- **Duration estimate:** L
- **Red:** Integration test: simulate disconnect, publish N events to the relay while down, reconnect, assert all N are eventually delivered without duplicates.
- **Green:** Track last-successfully-processed-event timestamp; populate `Since` in the `REQ` frame rebuilt on reconnect.
- **Refactor:** Confirm interaction with existing `seen`/`seenMu` dedupe is correct and doesn't need its own bound adjustment (cross-check against D.1's cache work if any overlap emerges).
- **Acceptance criteria:** See spec.md FR-010.

---

## Cross-Cluster Integration

### I.1 — Final quality gate and cluster merge verification
- **Dependencies:** A.1–A.8, B.1, C.1–C.3, D.1–D.2 all complete
- **Duration estimate:** S
- **Steps:** Merge all four clusters to the integration branch; run full quality gate (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run`, `go test ./...` including `-race`, `go build -o bin/nuimanbot ./cmd/nuimanbot`, `./bin/nuimanbot --help`); spot-check no regression in the original review's "Dimensions Reviewed With No Findings" areas.
- **Acceptance criteria:** All quality gates pass; `status.md` updated to 100% complete for all four clusters.
