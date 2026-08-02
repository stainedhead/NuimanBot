# Plan: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Status:** Planning

## Development Approach

Strict TDD (Red-Green-Refactor, AGENTS.md) per FR, executed in four clusters as defined by the review PRD's own file-overlap analysis. Clusters map to git worktrees / agent teammates for parallelism where files don't overlap; within a cluster, work is sequential to avoid merge conflicts on shared files (chiefly `gateway.go` in Cluster A, with Cluster B coordinating on the same file).

Fix order: **P0 → P1 → P2**, with P1 items (FR-001/002/003/004/005/007/010) completed before any P2 item (FR-008/009/011/012/013/014/016) starts, per AGENTS.md and the review PRD's guidance. Within Cluster A, this means FR-001/002/003 (P1, same lifecycle concern) go first, then FR-008/011/012/013/016 (P2) after all P1 work across all clusters lands — or, pragmatically, after Cluster A's own P1 items land, since Cluster A's P2 items don't block other clusters.

A brief code/design review follows each individual fix (not just each cluster), before starting the next one, per the review PRD's guidance — confirming the fix satisfies its acceptance criteria, doesn't regress any "Dimensions Reviewed With No Findings" area, and passes the full quality gate.

## Phase Breakdown

### Cluster A — `gateway.go` lifecycle & structure (single owner, sequential)
FR-001, FR-002, FR-003 (P1, do first) → FR-008, FR-011, FR-012, FR-013, FR-016 (P2, after). All touch the same file; one worktree/teammate, top-to-bottom by priority.

### Cluster B — Observability wiring (parallel with A, coordinates on gateway.go)
FR-004. Sets the gauge via `ConnectedRelayCount()` from the adapter layer. Coordinate timing with Cluster A to avoid simultaneous edits to `gateway.go`, or fold into Cluster A's sequence.

### Cluster C — Config & docs (parallel with A/B)
FR-005 → FR-007 (both touch `buzz-guide.md`, sequence within cluster) → FR-014 (different file, can run independently within the cluster).

### Cluster D — Cache & subscription (parallel with A/B/C)
FR-009 (`agent_cache.go`/`loop_guard.go`) and FR-010 (`subscription.go`) — different files within the cluster, can proceed in parallel with each other.

## Critical Path

Cluster A is the critical path (8 of 14 findings, single file, sequential). Clusters B/C/D can complete well ahead of Cluster A. Final integration/quality-gate re-run happens after all four clusters have merged back.

## Testing Strategy

- Every FR gets a dedicated failing test first (Red), per its acceptance criteria in spec.md.
- FR-008's fix is verified with `go test -race` specifically targeting concurrent `Send()`/`Start()`/`Stop()` calls.
- FR-010's fix is verified with a simulated disconnect/republish/reconnect integration test (real in-process WebSocket relay, consistent with the existing Buzz test suite's established pattern).
- FR-004's fix is verified with a test asserting gauge value changes on relay connect/disconnect.
- Full quality gate (`go fmt`, `go vet`, `golangci-lint run`, `go test ./...`, `go build -o bin/nuimanbot ./cmd/nuimanbot`, `./bin/nuimanbot --help`) runs after each cluster merges, per AGENTS.md.

## Rollout Strategy

No feature flag or staged rollout needed — these are reliability/observability/documentation fixes to an already-shipped-internally branch (`feat/nuimanbot-support-buzz`), not new user-facing behavior. Each cluster merges to the integration branch (this worktree's branch) after passing its own quality gate; the branch as a whole is re-verified before the PR is opened.

## Success Metrics

- 14/14 tasked FRs (all except deferred FR-006 and informational FR-015) closed with passing tests and satisfied acceptance criteria.
- `go test -race ./...` clean, including new FR-008 concurrency test.
- No regression detected in any of the review's "Dimensions Reviewed With No Findings" areas (spot-checked during each cluster's code review step).
- Full quality gate green at final integration.
