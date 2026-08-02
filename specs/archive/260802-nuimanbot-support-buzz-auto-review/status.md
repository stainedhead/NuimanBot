# Status: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Last updated:** 2026-08-02

## Overall Progress

| Phase | Description | Status | % Complete |
|---|---|---|---|
| Phase 0 | Spec creation | In Progress | — |
| Cluster A | gateway.go lifecycle & structure (FR-001/002/003/008/011/012/013/016) | Complete | 100% |
| Cluster B | Observability wiring (FR-004) | Complete | 100% |
| Cluster C | Config & docs (FR-005/007/014) | Complete | 100% |
| Cluster D | Cache & subscription (FR-009/010) | Complete | 100% |

## Phase 0 Task Checklist

- [x] Spec directory created
- [x] Review PRD's 14 fixable findings mapped to FR-001–005, FR-007–014, FR-016 in spec.md
- [x] FR-006 marked deferred, FR-015 marked informational-only (no task) in spec.md
- [x] Research questions identified (research.md notes design questions are pre-resolved by the review PRD)
- [x] All phase files initialized (spec.md, status.md, research.md, data-dictionary.md, architecture.md, plan.md, tasks.md, implementation-notes.md)
- [x] Review PRD moved into spec directory
- [x] spec.md References section updated to point at new PRD location

## Blockers

None currently.

## Recent Activity

- 2026-08-02: Spec directory `specs/260802-nuimanbot-support-buzz-auto-review/` created from review PRD. All phase files populated. 14 fixable findings (of 16 total) mapped to FRs; FR-006 deferred, FR-015 informational.
- 2026-08-02: Cluster D — FR-009 complete. `agentCache` now bounded by a sliding 24h TTL (refreshed on `Set`/`IsAgent`) plus a hard 5,000-entry capacity cap with oldest-evicted-first overflow handling. `loopGuard.channels` re-confirmed (not just assumed) to not need the same treatment — channel-keyed, bounded by static config, already ages out via its existing 30s window. Full rationale in implementation-notes.md. Tests: `TestAgentCache_TTLExpiry_StaleEntryEvictedAndTreatedAsUnknown`, `TestAgentCache_TTLIsSliding_RecentIsAgentTouchExtendsLife`, `TestAgentCache_CapacityBound_EvictsOldestWhenFull`, all passing under `-race`. Quality gate (fmt/vet/lint/test/build) green.
- 2026-08-02: FR-005 complete (Cluster C). `BuzzConfig.NIP05` doc comment marks it reserved for a future phase (PRD §6.5), not currently read. `support_docs/buzz-guide.md`'s `nip05` row updated to match `dm_policy`'s reserved/no-effect framing. Build verified.
- 2026-08-02: FR-007 complete (Cluster C). Chose to implement (scope stayed to a single bool): `applyEnvOverrides()` in `internal/config/loader.go` now supports `NUIMANBOT_GATEWAYS_BUZZ_ENABLED`, matching the existing `NUIMANBOT_GATEWAYS_CLI_DEBUGMODE` whitelist pattern. TDD Red-Green verified (test failed without the fix, passed with it). `buzz-guide.md` env-var section clarified to note only `enabled` has an env override; `relays`/`channel_ids` still require `config.yaml`. Full quality gate passed (fmt, tidy, vet, lint, test, build, run).
- 2026-08-02: FR-014 complete (Cluster C). `documentation/technical-details.md`'s stale `SkillPermissions` example (predating this branch) corrected to the current `ToolPermissions` variable name and role assignments (`calculator`/`datetime` now `RoleGuest`, matching `internal/usecase/tool/permissions.go`). Cluster C (FR-005/007/014) now 100% complete. Build verified.
- 2026-08-02: FR-010 complete (Cluster D). `nostr.Client` now tracks a per-relay high-water mark (`created_at` of the last event parsed from that relay) and rebuilds the REQ frame on every reconnect with `Filter.Since` set from it — the initial connect to each relay still omits `Since` (full backlog), matching prior behavior. Chose per-relay over gateway-wide tracking (relays may deliver out of order relative to each other — a shared mark could skip events on a slower relay) and exact-`created_at` (not `+1`) for `Since`, relying on `gateway.go`'s existing `seen`/`seenMu` dedupe to absorb the resulting harmless boundary re-delivery. Full rationale in implementation-notes.md. New test `TestClient_ReconnectBackfillsMissedEventsViaSince` simulates a drop, has the fake relay assert `since` on the reconnect REQ and deliver 3 "backfilled" events, and confirms all 4 events (1 pre-drop + 3 backfilled) are received. `go test -race -count=3` clean. Cluster D (FR-009/010) now 100% complete. Quality gate (fmt/vet/lint/test/build/run) green.
- 2026-08-02: Cluster A+B complete (FR-001/002/003/008/011/012/013/016, FR-004) — all in `internal/adapter/gateway/buzz/gateway.go` / `gateway_test.go`, plus `internal/infrastructure/metrics/prometheus.go` for FR-004 and FR-013. 9 separate commits, TDD red-green-refactor per fix, full quality gate (fmt/vet/lint/test/build/run) green after each, `go test -race ./internal/adapter/gateway/buzz/...` clean.
  - FR-001: nil-guard on `Send()` — descriptive error instead of panic when `g.client` is nil.
  - FR-002: `Stop()` test coverage + made `Stop()` idempotent (double-`Stop()` would otherwise panic via `nostr.Client.Stop()`'s repeat `close()` of its event channel — a `stopped`-flag guard added, later folded into FR-008's mutex).
  - FR-003: tested all three `Start()` error/guard paths. The `nostr.Client.Start` failure wrap was unreachable via any valid `Filter` built from `gateway.go`'s own inputs, so introduced a small `nostrClient` interface + `newNostrClient` factory var as a test seam. Also tightened `Start()` so `g.client` is only assigned after a successful `client.Start()` (previously assigned before, leaving a half-initialized client on failure).
  - FR-004: `buzz_relay_connections` gauge wired from the adapter layer via a `monitorRelayConnections` background poller (250ms) reading `client.ConnectedRelayCount()`, per the PRD's resolved decision. `ConnectedRelayCount()` only exposes an aggregate count (no per-relay URL/status), so re-scoped `BuzzRelayConnections` from a `{relay_url,status}`-labeled `GaugeVec` to a plain `Gauge` — the labels carried no information the adapter layer can actually supply.
  - FR-008: added an `RWMutex` guarding `client`/`messageHandler`/`cancel`/`stopped` (previously plain fields written from `Start()`/`OnMessage()` and read from `Send()`/`Stop()`/`handleEvents()` with no synchronization). New `-race`-run concurrency test calls `Send()`/`Stop()` concurrently with a live `Start()`.
  - FR-011: added `buzzUserService` interface (`GetUserByPlatformUID`/`CreateUser`), matching `chat.UserService`'s pattern; `Gateway`/`New()` now depend on it instead of the concrete `*user.Service`. `*user.Service` satisfies it, so `cmd/nuimanbot/main.go`'s construction call needed no change (confirmed via full `go build ./...`).
  - FR-012: added forged-signature tests for kind:9000 and kind:10100 (existing forgery tests only covered kind:9), plus `publishAgentProfileBestEffort` retry-exhaustion and `resolveUser` conflict/lookup-error branch tests.
  - FR-013: reworded `buzz_events_received_total`'s help text to state it counts kind:9 channel messages specifically (excludes kind:9000/kind:10100) — metric name/labels unchanged, no dashboard-breaking rename.
  - FR-016: **decided not to remove** the redundant Buzz-level `resolveUser` — full removal would require dropping `New()`'s `userService` parameter, which `cmd/nuimanbot/main.go`'s construction call depends on and was out of this task's file scope (`gateway.go`/`gateway_test.go`/`prometheus.go` only). Chose the finding's "document explicitly" remediation option instead: expanded doc comments on the `userService` field and `resolveUser` explaining the duplication with `chat.Service.resolveUser`, why it's harmless (return value discarded), and why it wasn't removed here. Flagged as safe to delete in a follow-up that also updates `main.go`.
  - Scope note: FR-013 and FR-004 both required editing `internal/infrastructure/metrics/prometheus.go`, which the task briefing scoped to "FR-004 only." Extended to FR-013 too since no other parallel cluster had any reason to touch that file and the edit was a single-line help-text change — flagged explicitly in the FR-013 commit message for visibility.
  - All 9 commits are on `feat/nuimanbot-support-buzz`, fast-forwarded cleanly on top of Cluster C/D's already-pushed work (no rebase conflicts encountered — this update was written after both other clusters had already completed and merged).
