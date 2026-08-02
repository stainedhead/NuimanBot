# Status: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Last updated:** 2026-08-02

## Overall Progress

| Phase | Description | Status | % Complete |
|---|---|---|---|
| Phase 0 | Spec creation | In Progress | — |
| Cluster A | gateway.go lifecycle & structure (FR-001/002/003/008/011/012/013/016) | Not Started | 0% |
| Cluster B | Observability wiring (FR-004) | Not Started | 0% |
| Cluster C | Config & docs (FR-005/007/014) | In Progress | 67% |
| Cluster D | Cache & subscription (FR-009/010) | Not Started | 0% |

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
- 2026-08-02: FR-005 complete (Cluster C). `BuzzConfig.NIP05` doc comment marks it reserved for a future phase (PRD §6.5), not currently read. `support_docs/buzz-guide.md`'s `nip05` row updated to match `dm_policy`'s reserved/no-effect framing. Build verified.
- 2026-08-02: FR-007 complete (Cluster C). Chose to implement (scope stayed to a single bool): `applyEnvOverrides()` in `internal/config/loader.go` now supports `NUIMANBOT_GATEWAYS_BUZZ_ENABLED`, matching the existing `NUIMANBOT_GATEWAYS_CLI_DEBUGMODE` whitelist pattern. TDD Red-Green verified (test failed without the fix, passed with it). `buzz-guide.md` env-var section clarified to note only `enabled` has an env override; `relays`/`channel_ids` still require `config.yaml`. Full quality gate passed (fmt, tidy, vet, lint, test, build, run).
