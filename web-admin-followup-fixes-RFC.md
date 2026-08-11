# RFC: Web Admin Follow-Up Fixes (GET-Triggered State Changes, Job Status Sync)

**Created:** 2026-08-11
**Status:** Draft
**Scope:** Two independent bugs discovered incidentally during the CLI-parity feature's code review (PR #10), both living in the six-environments feature merged as PR #8 (`main`, commit `52b69d5`). Neither was fixed at discovery time — both were deliberately deferred to this follow-up, per the repo owner's explicit direction.

---

## Executive Summary

Two real, independently-verified bugs exist in the currently-merged web admin (`internal/adapter/web`):

1. **State-changing actions (delete, message-send, schedule-confirm) are reachable via a plain GET request** carrying a valid CSRF token in the query string — no POST required. Affects Chats, Projects, Jobs, and Chores subroutes.
2. **`Job.Status` is never updated after creation** — every Job shows "queued" forever in the UI, even after its Run completes or fails. This silently breaks the Jobs list/detail views' observability promise and, worse, makes `DeleteJob` always take the soft-delete branch, so clicking Delete on a Job looks like it did nothing for up to 15 minutes (until the retention sweeper's next tick).

Both are scoped, low-risk fixes — neither requires new architecture, both fit within the existing Clean Architecture layering.

---

## Issue 1: GET-Triggered State Changes via CSRF Token in Query String

### Problem Statement

`validCSRF` (`internal/adapter/web/chats_handler.go:237`) reads the CSRF token via `r.FormValue("csrf_token")`. Go's `net/http` populates `r.Form` from the URL's query string on **every** request regardless of method, in addition to the POST body when applicable. Combined with the fact that none of `handleChatSubroutes`, `handleProjectSubroutes`, `handleJobSubroutes`, or `handleChoreSubroutes` check `r.Method` before dispatching to a delete/message/confirm action, a plain `GET` request with a valid token in the query string triggers the same state change a POST form submission would:

```
GET /admin/projects/{id}/delete?csrf_token=<valid-token>
```

...deletes the Project. No form submission, no POST body — just a link.

**Verified affected routes** (subroutes dispatcher has no `r.Method` check before a state-changing action):
- `internal/adapter/web/chats_handler.go` — `handleChatSubroutes`: `delete`, `message` (send)
- `internal/adapter/web/projects_handler.go` — `handleProjectSubroutes`: `delete`, `add-agents-file`
- `internal/adapter/web/jobs_handler.go` — `handleJobSubroutes`: `delete`
- `internal/adapter/web/chores_handler.go` — `handleChoreSubroutes`: `delete`, `confirm` (schedule confirmation)

**Verified NOT affected** (already method-gated correctly):
- `internal/adapter/web/memories_handler.go` — checks `r.Method != http.MethodGet` / `!= http.MethodPost` explicitly at each relevant handler.
- `internal/adapter/web/history_handler.go` — `handleHistorySubroutes` has no state-changing action (detail view only; marking a run "viewed" is an idempotent, non-destructive side effect of viewing, not a CSRF-relevant action).
- `internal/adapter/web/settings_handler.go` — `handleSettings` gates its update path with `if r.Method == http.MethodPost`.

**Impact:** This is a defense-in-depth gap, not a full CSRF bypass — the attacker still needs a valid token, which isn't trivially guessable. But a token can leak via `Referer` header forwarding, browser link-prefetching, proxy/CDN access logs, or a crawler following an unlucky link — any of which normally would NOT trigger a state change for a properly method-gated endpoint, since GET requests are supposed to be safe/idempotent by HTTP convention. An attacker who obtains a leaked token (e.g., via a misconfigured `Referer` from an internal tool, or a shared proxy log) can trigger a destructive action with a single unauthenticated link click if the browser happens to still hold a valid session — no form interaction needed.

### Proposed Fix

Add an `r.Method != http.MethodPost` early-return (`http.NotFound` or `http.Error(w, ..., http.StatusMethodNotAllowed)`) at the top of each of the four affected subroutes dispatchers (`handleChatSubroutes`, `handleProjectSubroutes`, `handleJobSubroutes`, `handleChoreSubroutes`) for the specific state-changing `case` branches (`delete`, `message`, `add-agents-file`, `confirm`) — not the read-only `case ""` (detail view), which should remain GET-accessible.

**FR-001:** `handleChatSubroutes`'s `delete` and `message` cases return 404 (matching the existing not-found convention for invalid routes, to avoid disclosing route existence via a distinct status code) for any non-POST request.

**FR-002:** `handleProjectSubroutes`'s `delete` and `add-agents-file` cases return 404 for any non-POST request.

**FR-003:** `handleJobSubroutes`'s `delete` case returns 404 for any non-POST request.

**FR-004:** `handleChoreSubroutes`'s `delete` and `confirm` cases return 404 for any non-POST request.

**FR-005:** Existing templates (`chats.html`, `projects.html`, `jobs.html`, `chores.html` and their detail-page variants) already submit these actions via `<form method="POST">` per the existing CSRF-token-in-form convention — verify no template relies on a GET link for any of these four actions before landing the fix (a genuine GET-based delete link, if one exists, would need updating to a form submission first).

### Acceptance Criteria

- [ ] `curl -X GET "http://localhost:PORT/admin/projects/{id}/delete?csrf_token=<valid>"` (with a valid session cookie) returns 404, and the Project still exists afterward.
- [ ] Equivalent GET-based checks for Chats (`delete`, `message`), Jobs (`delete`), and Chores (`delete`, `confirm`) all return 404 without performing the action.
- [ ] The existing POST-based flows (form submission via the actual UI) continue to work unchanged — no regression in the normal user-facing delete/confirm/send flows.
- [ ] Existing handler test suites (`chats_handler_test.go`, `projects_handler_test.go`, `jobs_handler_test.go`, `chores_handler_test.go`) gain a `TestX_YAction_RejectsNonPOST`-style test per affected action.

---

## Issue 2: `Job.Status` Never Syncs With Its Run's Actual Lifecycle

### Problem Statement

`domain.JobRepository.UpdateStatus` (`internal/domain/job_repository.go:28`) is defined on the interface and implemented (`internal/infrastructure/storage/file_job_repository.go:161`), but **no production code path calls it**. `internal/infrastructure/scheduler/stub_executor.go`'s `Execute` drives the associated `Run`'s status through `Running → Completed/Failed` via three `SaveRun` calls, but never updates the source `Job`'s `Status` field. Every Job's `Status` remains whatever `CreateJob` set at creation time (`domain.JobStatusQueued`, `internal/usecase/jobs/service.go:139`) forever — even after the Job's Run has long since finished.

This is a known, partially-acknowledged gap: a comment at `internal/usecase/jobs/service.go:259` (inside `CleanupPendingDeletion`) explicitly documents that "`Job.Status` is not currently kept in sync with its Run's actual lifecycle (no caller ever invokes `JobRepository.UpdateStatus` in this codebase today)," and works around it there by deriving activeness from the Run directly (`hasActiveRun`, which queries `RunRepository`) rather than trusting `Job.Status`.

**But that workaround was only applied to `CleanupPendingDeletion` (the background retention sweep), not to `DeleteJob` itself** (`internal/usecase/jobs/service.go:210-226`, the user-facing delete path):

```go
func (s *Service) DeleteJob(ctx context.Context, ownerUserID, jobID string) error {
	job, err := s.jobs.GetJob(ctx, ownerUserID, jobID)
	...
	if job.Status == domain.JobStatusRunning || job.Status == domain.JobStatusQueued {
		job.PendingDeletion = true
		...
		return nil
	}
	return s.jobs.DeleteJob(ctx, ownerUserID, jobID)
}
```

Since `job.Status` is always `JobStatusQueued` (finding above), this branch is **always** taken — `DeleteJob` never reaches the immediate hard-delete path, regardless of whether the Job's Run actually finished minutes or hours ago. The user clicks Delete, gets redirected to the Jobs list, and the Job... is still there, indistinguishable from a live job (no `PendingDeletion` indicator in `jobs.html`'s list view — only the detail page shows "(pending deletion)"). It silently disappears up to 15 minutes later when the retention sweeper's next tick runs `CleanupPendingDeletion`.

**Secondary user-facing symptom:** `jobs.html` and `job_detail.html` render `.Status` directly, so every Job in the UI shows "queued" forever, even completed/failed ones — users must cross-reference the History environment to see real outcomes, undercutting the Jobs environment's own observability promise (FR-028/FR-030).

### Proposed Fix

Two coordinated changes:

**FR-006:** Wire `JobRepository.UpdateStatus` into the actual Run lifecycle. `internal/infrastructure/scheduler/stub_executor.go`'s `Execute` (or whatever calls it — the worker pool's dispatch point) should update the source Job's `Status` alongside each `SaveRun` call: `JobStatusRunning` when the Run starts, and a terminal status (`JobStatusCompleted`/`JobStatusFailed` — confirm the exact `domain.JobStatus` constants already defined) when the Run finishes. This must be `ownerUserID`-scoped, matching `UpdateStatus`'s existing signature.

**FR-007:** Fix `DeleteJob` to derive "is this Job's work still active" the same way `CleanupPendingDeletion` already does — via `hasActiveRun` (querying `RunRepository` directly) — rather than trusting the (now-fixed, but still indirect) `Job.Status` field. This keeps a single source of truth for "active" and avoids the two methods drifting again if `Status` sync has some future edge-case gap.

**FR-008:** Add a `PendingDeletion` visual indicator to `jobs.html`'s list view (not just the detail page), so a user who deletes a still-running Job understands why it's still listed, rather than concluding "delete didn't work."

### Acceptance Criteria

- [ ] After a Job's Run completes (via the existing `StubExecutor` path), `GET /admin/jobs/{id}` shows a real terminal status (not "queued"), and `jobs.html`'s list view reflects it too.
- [ ] `DeleteJob` on a Job whose Run has already reached a terminal state hard-deletes it immediately (no 15-minute wait, no `PendingDeletion` state) — verified via a test that creates a Job, lets its stub Run complete, then deletes it and asserts it's gone from `ListJobs` immediately.
- [ ] `DeleteJob` on a Job whose Run is still `Running`/`Queued` still soft-deletes (unchanged behavior for the genuinely-active case) — existing tests for this path must continue passing.
- [ ] `jobs.html`'s list view shows a `PendingDeletion` badge/indicator for any Job in that state, matching `job_detail.html`'s existing convention.
- [ ] `CleanupPendingDeletion`'s existing behavior is unaffected (still uses `hasActiveRun`; this fix doesn't need to touch that method, just bring `DeleteJob` in line with it).

---

## Dependencies and Risks

| Item | Type | Notes |
|---|---|---|
| `internal/infrastructure/scheduler/stub_executor.go` | Dependency | FR-006 modifies the same executor that a future real (non-stub) agent-invoking executor will eventually replace — the `UpdateStatus` wiring should live at a point both the stub and its eventual replacement will share (e.g. the worker pool's dispatch/completion hooks), not duplicated inside `StubExecutor` itself if that's avoidable. |
| Existing `jobs_handler_test.go`, `service_test.go` | Dependency | Both already have decent coverage (93%+ per the CLI-parity review) — new tests should extend, not restructure, existing suites. |
| Templates (`chats.html`, `projects.html`, `jobs.html`, `chores.html`) | Risk | FR-005 requires verifying no existing GET-based link relies on the current (buggy) permissive behavior before landing Issue 1's fix — a missed template would silently break a legitimate user flow, not just close a security gap. |
| CI reliability (flaky integration tests) | Note | The CLI-parity PR (#10) hit CI-only test timeouts caused by `bcryptCost=12` + `-race` overhead in `internal/adapter/gateway/cli`'s integration tests (fixed there by widening timeouts). Any new tests for this RFC that spin up goroutines with tight timeouts should budget generously (10s+) for CI, not just local dev speed. |

## Open Questions

- Should Issue 1's fix use a plain `http.NotFound` (matching the existing not-found convention, avoiding a distinct "method not allowed but route exists" signal) or `http.StatusMethodNotAllowed` (more semantically correct per HTTP, but discloses the route exists via a different status code than a genuinely-missing route)? Recommend `http.NotFound` for consistency with this repo's existing "don't disclose via status code" convention (see Edge Case #10 in the original feature's spec).
- **Resolved:** `domain.JobStatus` constants confirmed against `internal/domain/job.go`: `JobStatusQueued`, `JobStatusRunning`, `JobStatusCompleted`, `JobStatusFailed` — FR-006 can use these directly, no further verification needed.
- Whether this RFC should go through the full `dev-flow` PRD → spec → implementation pipeline (matching how the two prior features were built) or be implemented directly as a smaller, single-pass fix, given its scope is materially smaller than either prior feature.
