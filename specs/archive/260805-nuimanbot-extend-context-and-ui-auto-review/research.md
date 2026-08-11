# Research: NuimanBot Extend Context & UI — Auto-Review Fix Pass

**Created:** 2026-08-05
**Source PRD:** [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md) — Open Questions section

## Research Questions

These are the PRD's own Open Questions, carried forward verbatim with their stated defaults. Per the PRD ("no human is available mid-pipeline for follow-up"), each is a decision-with-default the fix pass proceeds under, not a blocking research task for a separate party to resolve first — but each still needs its answer traced/recorded as part of closing the relevant finding.

1. **(FR-R7) Is memory cell `ConversationID` keyed per-user or per-session for a given gateway?**
   *Default:* treat as unverified; FR-R7's acceptance criteria (trace one real gateway end-to-end from message receipt through `curator_service.go` to persisted `MemoryCell.ConversationID`, add an integration test) **is** the resolution mechanism — not a prerequisite someone else answers first.
   *Action:* trace at least one real gateway (CLI or Telegram suggested in the finding) as part of implementing FR-R7.

2. **(FR-R1 / FR-R4) Implement the agent-facing surface, or defer with documentation + UI notice?**
   *Default:* attempt the "implement" path first (preferred per both findings' acceptance criteria). If genuinely blocked by the scope of `internal/usecase/chat` integration, fall back to "defer + document in `implementation-notes.md` + UI notice."
   *Note:* FR-R4 (per-item chat) does **not** get the documentation-only fallback — at least one of Job/Chore/Run/Memory chat must actually be implemented as a template. Only FR-R1 (the full Chats reply loop) has the fallback option.
   *Action:* record whichever outcome is chosen, explicitly, in `implementation-notes.md`'s "Deviations from Plan" — the PRD's own finding (FR-R1) flags that this choice was made silently once already for the Jobs/Chores `StubExecutor` deferral and must not repeat that omission.

3. **(FR-R11) Is a live network-listener rebind in scope for this pass?**
   *Default:* no — out of scope. The fix only needs to stop the Settings UI from presenting config-file-only fields (allowlist, bind address) as if they were live-editable.
   *Action:* confirm during implementation whether extending the POST handler to accept+apply allowlist/bind-address changes (with actual listener rebind) is still judged out of scope, or whether the simpler "grey out non-live fields" path is taken.

4. **(FR-R18) What is the correct allowed root for Project output directories?**
   *Default:* confine under `<storagePath>/users/<ownerUserID>/projects/`, matching the existing per-user storage convention already used by Chats/Jobs/Chores/History.
   *Action:* before implementing, check `internal/config` for any existing per-deployment "projects root" config value already wired elsewhere; only fall back to the default convention if none exists.

## Additional Research Notes (from PRD investigation, already resolved)

The PRD itself is the product of an already-completed investigation (grep-verified call sites, traced doc comments, confirmed test coverage gaps) — most of the "research" for this fix pass is already done and embedded in each finding's text. The items below are execution-planning questions specific to *this* spec, not re-litigations of the findings:

- **Workstream file-collision coordination:** `projects/service.go` is touched by both FR-R5 (workstream B, relocate `fsguard` calls) and FR-R18 (workstream B, root confinement) — the PRD specifies FR-R5 lands first, FR-R18 rebases against it before merging. Confirm this ordering holds once implementation starts; if FR-R18 is trivially separable from the `fsguard`-relocation diff, note that in `plan.md`.
- **`server.go`'s `BaseData` construction** is a shared edit surface for FR-R10 (workstream E) and FR-R19 (workstream E) — both in the same workstream already, so no cross-workstream coordination needed, only within-workstream sequencing awareness.
- **Sweep-loop reuse:** FR-R3 (workstream A) builds the periodic sweep scaffold; FR-R9 (workstream A) is explicitly stated by the PRD as combinable with the same loop rather than a second independent ticker. Confirm the `ChoreScheduler` poll-loop pattern referenced in FR-R3's acceptance criteria is the right template to extend for FR-R9's `PendingDeletion` check.

## Industry Standards / Existing Implementations / API Documentation / Best Practices

Not applicable in the conventional sense — all 19 findings are internal-codebase gaps against this project's own prior spec, not new integrations requiring external API research. Relevant internal precedent to reuse:

- `ChoreScheduler`'s poll-loop pattern (referenced by FR-R3, FR-R9) — existing periodic-execution pattern to extend rather than reinvent.
- `Queue`'s restart-durability test pattern (`TestQueue_RestartDurability`, `TestQueue_RestartDurability_AfterPartialDequeue`) — the pattern FR-R2's new test should extend to in-flight Run state.
- `jobs.Service.DeleteJob`'s existing soft-delete implementation — the exact pattern FR-R8 mirrors for Chores.
- The four new file repositories' `fsguard.ResolveWithin` usage pattern — the pattern FR-R13 extends to `file_conversation_repository.go`.

## References

- Source PRD: [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md), "Open Questions" section (lines ~317–324) and "Fix-Pass Execution Guidance" section (lines ~289–313).
- Original feature spec (archived): `specs/archive/260805-nuimanbot-extend-context-and-ui/spec.md`, `research.md`.
