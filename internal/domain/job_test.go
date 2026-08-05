package domain

import "testing"

func TestJob_IsQueueable_FreshJob(t *testing.T) {
	j := &Job{Status: JobStatusCompleted}
	if !j.IsQueueable() {
		t.Fatal("expected a completed, non-pending-deletion job to be queueable for a new run")
	}
}

func TestJob_IsQueueable_AlreadyRunning(t *testing.T) {
	j := &Job{Status: JobStatusRunning}
	if j.IsQueueable() {
		t.Fatal("expected an already-running job to not be queueable")
	}
}

func TestJob_IsQueueable_AlreadyQueued(t *testing.T) {
	j := &Job{Status: JobStatusQueued}
	if j.IsQueueable() {
		t.Fatal("expected an already-queued job to not be queueable")
	}
}

func TestJob_IsQueueable_PendingDeletion(t *testing.T) {
	// Edge Case #3: no new runs are enqueued for a Job pending deletion.
	j := &Job{Status: JobStatusCompleted, PendingDeletion: true}
	if j.IsQueueable() {
		t.Fatal("expected a job pending deletion to never be queueable")
	}
}
