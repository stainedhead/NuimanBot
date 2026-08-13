# Research: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11
**Source PRD:** `cli-parity-for-nuimanbot-features-auto-review-PRD.md` (in this directory)

## Research Questions

Seeded from the PRD's Open Questions and Dependencies sections:

1. **FR-002 (Chat-context ownership check):** Should Chat-context ownership rejection reuse the same best-effort "swallow lookup error, leave field unresolved" pattern `ContextTypeProject` uses today, or should Chat get a harder rejection (return an error to the CLI immediately)? No "stale/deleted, owned" tolerance test currently exists for Chat to establish a soft-fail precedent, unlike Project. Left to the implementing workstream to decide and document — spec.md's line-157 criterion only requires rejection, not a specific failure mode.
2. **FR-004's optional follow-up:** Which structural fix is preferred if picked up — refuse-to-run-without-authHandler outside test builds, or change `defaultRoleForPlatform(PlatformCLI)`'s default away from `RoleAdmin`? Not decided by the PRD; only the documentation requirement (not the optional fix itself) is in scope for this pass.
3. **FR-005's tracked follow-up FR:** Should CLI and web failed-login audit logging ship as one combined FR or two independent ones? They touch different files (`cli/auth_commands.go` vs `web/auth.go`) with no shared code path apart from the `internal/infrastructure/audit` package itself. Not decided here — filing the FR (in whichever shape) is what's required to close FR-005.
4. **StubExecutor's eventual replacement:** No FR in this PRD tracks the follow-up work of a real executor re-verifying Chat-context ownership at read time. Confirm whether a placeholder tracking issue/FR should be filed now (outside this pass's scope) so it isn't lost when the stub is eventually replaced.
5. **FR-002 test pattern:** Confirm `TestGetJob_CrossOwnerIsolation`'s existing pattern (referenced in the PRD as the model to mirror) is directly reusable for the new cross-owner `CreateJob` test, or whether `CreateJob`'s different call signature (context ID passed at creation vs. looked up post-creation) requires a materially different test harness.

## Industry Standards

[TBD] — not directly applicable; these are internal code-review remediation items, not new external-facing functionality requiring industry-standard research.

## Existing Implementations

- `/history list`'s `historyListDisplayLimit = 20` and `/history show`'s `historyContentPreviewLimit = 4000` (`internal/adapter/gateway/cli/history_commands.go`) — the established truncation/pagination pattern FR-001 must match.
- `internal/adapter/web/memories_handler.go`'s `sortCellsByCreatedAtDesc` — the display-parity sort pattern referenced as FR-001's optional follow-up.
- Settings' deferred-subcommand messaging convention (`retentionSetNotImplementedMessage`, `networkModeNotImplementedMessage` in `settings_commands.go`) — the pattern FR-003 must match for the four deferred `chat` subcommands.
- `TestGetJob_CrossOwnerIsolation` and its sibling cross-owner tests (`TestDeleteJob_CrossOwnerIsolation`, `TestGetProject_CrossOwnerIsolation`, etc., listed in the PRD's Positive Observations) — established usecase-layer cross-owner test pattern to mirror for FR-002.
- `internal/infrastructure/audit`'s `AuditLogger` type — exists in the codebase but is currently unused by `internal/adapter/web/auth.go`; the target for FR-005's follow-up FR.

## API Documentation

Not applicable — no external API integration involved in this fix pass.

## Best Practices

- AGENTS.md's mandatory Red-Green-Refactor cycle applies to every finding with a code change (FR-001, FR-002, FR-003, FR-007).
- AGENTS.md's quality-gate sequence (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run`, `go test ./...`, `go build`, `./bin/nuimanbot --help`) must pass after every individual fix per the PRD's Fix Process Guidance, not just once at the end.
- `dev-flow:review-code` should run per individual fix rather than batched, per the PRD's explicit guidance — this keeps a bad fix from compounding into the next one.

## Open Questions

See Research Questions above — all four PRD-sourced open questions are captured there and remain unresolved by design (left to the implementing workstream per the PRD).

## References

- Source PRD: `cli-parity-for-nuimanbot-features-auto-review-PRD.md` (this directory)
- Original archived spec: `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md`
