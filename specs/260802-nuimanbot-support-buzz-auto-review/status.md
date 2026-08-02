# Status: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Last updated:** 2026-08-02

## Overall Progress

| Phase | Description | Status | % Complete |
|---|---|---|---|
| Phase 0 | Spec creation | In Progress | — |
| Cluster A | gateway.go lifecycle & structure (FR-001/002/003/008/011/012/013/016) | Not Started | 0% |
| Cluster B | Observability wiring (FR-004) | Not Started | 0% |
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
