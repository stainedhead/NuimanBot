package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

// errProjectDeleted is the exact Edge Case #2 error message a Run must be
// failed with when its source Job/Chore references a Project that has
// since been deleted.
var errProjectDeleted = errors.New("referenced Project no longer exists")

// StubExecutor is a functional, non-LLM-invoking Executor: it drives a Run
// through its real lifecycle (Running -> Completed/Failed), appending log
// lines and writing a RESULTS.md via fsguard, WITHOUT calling out to the
// agent/LLM. It exists so the rest of the pipeline — History, notification
// badges, WebSocket status push, worker pool bookkeeping — has a genuine,
// demonstrable execution path end-to-end in this pass, rather than every
// Job/Chore run sitting at Queued forever.
//
// Replacing this with a concrete agent-invoking Executor (deferred — see
// implementation-notes.md's "Deviations from Plan") is a drop-in swap:
// nothing else in this package depends on StubExecutor specifically, only
// on the Executor interface.
//
// FR-002 (auto-review fix pass) obligation for that future executor:
// jobs.Service.CreateJob now verifies Chat-context ownership before a Job
// is persisted, so a Job's ContextID can no longer be seeded with a
// foreign-owned Chat at creation time. That guarantee holds only at
// creation time, though — StubExecutor never dereferences Chat content
// (only checkProjectExists below reads anything, and only for
// ContextTypeProject), so today there is no run-time re-check needed. A
// real executor that reads Chat content via a Run's source Job/Chore
// ContextID at execution time MUST re-verify that Chat is still owned by
// the Run's OwnerUserID before reading it (mirroring checkProjectExists'
// pattern for Project) — CreateJob-time verification cannot protect
// against a Chat's ownership changing (or the check being bypassed via a
// different creation path) between Job creation and eventual execution.
// See specs/260811-cli-parity-for-nuimanbot-features-auto-review's FR-002
// finding and its Open Questions ("StubExecutor's eventual replacement")
// for the full rationale.
type StubExecutor struct {
	runs     domain.RunRepository
	jobs     domain.JobRepository
	chores   domain.ChoreRepository
	projects domain.ProjectRepository
	baseDir  string // root directory for run artifacts: <baseDir>/users/<ownerUserID>/runs/<runID>/
}

// NewStubExecutor creates a StubExecutor. baseDir is the root under which
// each run's RESULTS.md is written, confined via fsguard.ResolveWithin.
// jobs/chores/projects back the FR-R12/Edge Case #2 check: before running,
// a Project-scoped Job/Chore's referenced Project must still exist.
func NewStubExecutor(runs domain.RunRepository, jobs domain.JobRepository, chores domain.ChoreRepository, projects domain.ProjectRepository, baseDir string) *StubExecutor {
	return &StubExecutor{runs: runs, jobs: jobs, chores: chores, projects: projects, baseDir: baseDir}
}

// Execute implements Executor.
func (e *StubExecutor) Execute(ctx context.Context, req RunRequest) {
	run, err := e.runs.GetRun(ctx, req.OwnerUserID, req.RunID)
	if err != nil {
		slog.Error("stub executor: failed to load run", "runID", req.RunID, "error", err)
		return
	}

	started := time.Now()
	run.Status = domain.RunStatusRunning
	run.StartedAt = &started
	if err := e.runs.SaveRun(ctx, run); err != nil {
		slog.Error("stub executor: failed to save running status", "runID", req.RunID, "error", err)
	}
	e.appendLog(ctx, req, fmt.Sprintf("[%s] run started (source=%s/%s)\n", started.Format(time.RFC3339), req.SourceType, req.SourceID))

	if ctxErr := ctx.Err(); ctxErr != nil {
		e.finishFailed(ctx, req, run, ctxErr)
		return
	}

	if err := e.checkProjectExists(ctx, req); err != nil {
		e.finishFailed(ctx, req, run, err)
		return
	}

	resultsPath, err := e.writeResults(req)
	if err != nil {
		e.finishFailed(ctx, req, run, err)
		return
	}

	ended := time.Now()
	run.Status = domain.RunStatusCompleted
	run.EndedAt = &ended
	run.ResultsPath = resultsPath
	if err := e.runs.SaveRun(ctx, run); err != nil {
		slog.Error("stub executor: failed to save completed status", "runID", req.RunID, "error", err)
	}
	e.appendLog(ctx, req, fmt.Sprintf("[%s] run completed\n", ended.Format(time.RFC3339)))
}

// finishFailed records a run failure (Edge Case #13: an LLM/agent failure —
// or here, a stub-execution failure — is treated identically to any other
// run failure: Status=Failed, Error populated, partial log preserved).
func (e *StubExecutor) finishFailed(ctx context.Context, req RunRequest, run *domain.Run, cause error) {
	ended := time.Now()
	msg := cause.Error()
	run.Status = domain.RunStatusFailed
	run.EndedAt = &ended
	run.Error = &msg
	if err := e.runs.SaveRun(ctx, run); err != nil {
		slog.Error("stub executor: failed to save failed status", "runID", req.RunID, "error", err)
	}
	e.appendLog(ctx, req, fmt.Sprintf("[%s] run failed: %s\n", ended.Format(time.RFC3339), msg))
}

// checkProjectExists implements FR-R12/Edge Case #2: if req's source Job or
// Chore runs in the context of a Project, that Project must still exist. A
// source record that can't itself be resolved (e.g. no Job/Chore repository
// wired, or the record is otherwise missing) is not this check's concern —
// it simply proceeds, since that is a distinct failure mode from "the
// Project was deleted".
func (e *StubExecutor) checkProjectExists(ctx context.Context, req RunRequest) error {
	contextType, contextID, ok := e.resolveContext(ctx, req)
	if !ok || contextType != domain.ContextTypeProject {
		return nil
	}
	if _, err := e.projects.GetProject(ctx, req.OwnerUserID, contextID); err != nil {
		return errProjectDeleted
	}
	return nil
}

// resolveContext looks up req's source Job or Chore to determine its
// ContextType/ContextID. ok is false if the source can't be resolved (no
// repository wired, or the record is missing).
func (e *StubExecutor) resolveContext(ctx context.Context, req RunRequest) (contextType domain.ContextType, contextID string, ok bool) {
	switch req.SourceType {
	case domain.SourceTypeJob:
		j, err := e.jobs.GetJob(ctx, req.OwnerUserID, req.SourceID)
		if err != nil {
			return "", "", false
		}
		return j.ContextType, j.ContextID, true
	case domain.SourceTypeChore:
		c, err := e.chores.GetChore(ctx, req.OwnerUserID, req.SourceID)
		if err != nil {
			return "", "", false
		}
		return c.ContextType, c.ContextID, true
	default:
		return "", "", false
	}
}

// appendLog is a best-effort log append; a logging failure must never mask
// or interrupt the run's actual status transition.
func (e *StubExecutor) appendLog(ctx context.Context, req RunRequest, line string) {
	if err := e.runs.AppendLog(ctx, req.OwnerUserID, req.RunID, line); err != nil {
		slog.Error("stub executor: failed to append log", "runID", req.RunID, "error", err)
	}
}

// writeResults writes a placeholder RESULTS.md for req, confined under
// e.baseDir via fsguard.ResolveWithinNoEscape (FR-R6: also guards against a
// symlink-based escape) — demonstrating the same path-confinement
// discipline required of any real Job/Chore file operation (spec.md's
// Security NFR), even though this stub never touches a Project's actual
// output directory.
func (e *StubExecutor) writeResults(req RunRequest) (string, error) {
	runDir := filepath.Join(e.baseDir, "users", req.OwnerUserID, "runs", req.RunID)
	resultsPath, err := fsguard.ResolveWithinNoEscape(runDir, "RESULTS.md")
	if err != nil {
		return "", fmt.Errorf("failed to resolve results path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(resultsPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create run directory: %w", err)
	}

	content := fmt.Sprintf(
		"# Run Results\n\n- Source: %s %s\n- Run ID: %s\n\n_Executed by StubExecutor — no agent/LLM invocation occurred. See implementation-notes.md._\n",
		req.SourceType, req.SourceID, req.RunID,
	)
	if err := os.WriteFile(resultsPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write results file: %w", err)
	}

	return resultsPath, nil
}

var _ Executor = (*StubExecutor)(nil)
