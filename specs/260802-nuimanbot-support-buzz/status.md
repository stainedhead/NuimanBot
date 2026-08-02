# Status: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Last Updated:** 2026-08-02

## Overall Progress

| Phase | Description | Status | Progress |
|---|---|---|---|
| Phase 0 | Spec creation + review | Complete | 100% |
| Phase 1 | Read-only participation (FR-001..FR-007) | Not Started | 0% |
| Phase 2 | Full gateway — Send() + loop prevention (FR-008, FR-009) | Not Started | 0% |
| Phase 3 | Tool integration (FR-010) | Not Started | 0% |

## Phase 0 Task Checklist

- [x] Spec directory created (`specs/260802-nuimanbot-support-buzz/`)
- [x] spec.md populated from PRD (FRs, NFRs, acceptance criteria carried over from PRD §6.9/§6.10/§8)
- [x] Research questions identified (seeded in research.md, including Nostr library spike)
- [x] Phase files initialized (research, data-dictionary, architecture, plan, tasks, implementation-notes)
- [x] PRD moved into spec directory (`nuimanbot-support-buzz-PRD.md`)
- [x] Spec reviewed (`/review-spec`) — 2026-08-02, verdict: ready for implementation after fixes (see Recent Activity)

## Blockers

Phase 1 coding is blocked on **two** P0 spikes (both scoped in tasks.md), not one:
- **P0.1** — Nostr client/signing library choice (research.md Q1).
- **P0.2** — Buzz protocol conventions: event kind, channel tag, agent-identity tag (research.md Q5). Found missing a dedicated task during spec review; added.

Phase 3 (P3.1) is additionally blocked on a design decision: research.md Q6 (tool-execution RBAC-enforcement gap, found during spec review — `usecase/chat/tool_conversion.go` currently bypasses `ExecuteWithUser`'s RBAC/rate-limit checks for all platforms, not just Buzz). Does not block Phase 1/2.

## Recent Activity

- 2026-08-02: Spec directory created from `nuimanbot-support-buzz-PRD.md`. All phase files initialized. PRD content (FRs, NFRs, rollout exit criteria) carried into spec.md without re-derivation, per explicit instruction — this PRD was already reviewed/hardened for this purpose.
- 2026-08-02: Spec review completed (`/review-spec`). Verified claims against the actual codebase and fixed several gaps directly: (1) added tasks.md P0.2 spike for research.md Q5 (was previously unscoped, silently deferred into P1.3's refactor step); (2) added tasks.md P1.6b — FR-007's generate-if-absent keypair helper had zero task coverage; added corresponding Phase 1 exit criterion to spec.md; (3) corrected the `"buzz:<pubkey>"` user-key convention across spec.md/data-dictionary.md/architecture.md/tasks.md — `domain.User.ID` is a UUID, lookup is by `(Platform, PlatformUID)` tuple via the existing `usecase/user.Service`, and no gateway currently auto-creates users on first message (that claim was inaccurate — CreateUser today is CLI-admin-only); (4) clarified `BuzzConfig.DMPolicy` is reserved/unused — no FR in this spec covers Buzz DMs; (5) flagged (not resolved) research.md Q6: Phase 3's "no bypass" RBAC claim can't be verified as-is because the chat-triggered tool path bypasses RBAC/rate-limit checks for all platforms today, not just Buzz — needs a design decision before P3.1 starts.
