package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

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
type StubExecutor struct {
	runs    domain.RunRepository
	baseDir string // root directory for run artifacts: <baseDir>/users/<ownerUserID>/runs/<runID>/
}

// NewStubExecutor creates a StubExecutor. baseDir is the root under which
// each run's RESULTS.md is written, confined via fsguard.ResolveWithin.
func NewStubExecutor(runs domain.RunRepository, baseDir string) *StubExecutor {
	return &StubExecutor{runs: runs, baseDir: baseDir}
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

// appendLog is a best-effort log append; a logging failure must never mask
// or interrupt the run's actual status transition.
func (e *StubExecutor) appendLog(ctx context.Context, req RunRequest, line string) {
	if err := e.runs.AppendLog(ctx, req.OwnerUserID, req.RunID, line); err != nil {
		slog.Error("stub executor: failed to append log", "runID", req.RunID, "error", err)
	}
}

// writeResults writes a placeholder RESULTS.md for req, confined under
// e.baseDir via fsguard.ResolveWithin — demonstrating the same path-
// confinement discipline required of any real Job/Chore file operation
// (spec.md's Security NFR), even though this stub never touches a Project's
// actual output directory.
func (e *StubExecutor) writeResults(req RunRequest) (string, error) {
	runDir := filepath.Join(e.baseDir, "users", req.OwnerUserID, "runs", req.RunID)
	resultsPath, err := fsguard.ResolveWithin(runDir, "RESULTS.md")
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
