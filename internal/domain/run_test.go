package domain

import (
	"testing"
	"time"
)

func TestRunStatus_IsTerminal(t *testing.T) {
	terminal := []RunStatus{RunStatusCompleted, RunStatusFailed, RunStatusSkipped}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}
	nonTerminal := []RunStatus{RunStatusQueued, RunStatusRunning}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected %q to not be terminal", s)
		}
	}
}

func TestRun_Duration_NotStarted(t *testing.T) {
	r := &Run{}
	if d := r.Duration(); d != 0 {
		t.Fatalf("expected 0 duration for a run that hasn't started, got %v", d)
	}
}

func TestRun_Duration_StartedNotEnded(t *testing.T) {
	started := time.Now()
	r := &Run{StartedAt: &started}
	if d := r.Duration(); d != 0 {
		t.Fatalf("expected 0 duration for a run still in flight, got %v", d)
	}
}

func TestRun_Duration_Completed(t *testing.T) {
	started := time.Now()
	ended := started.Add(90 * time.Second)
	r := &Run{StartedAt: &started, EndedAt: &ended}
	if d := r.Duration(); d != 90*time.Second {
		t.Fatalf("expected 90s duration, got %v", d)
	}
}

func TestRun_IsUnviewed(t *testing.T) {
	r := &Run{Status: RunStatusCompleted}
	if !r.IsUnviewed() {
		t.Fatal("expected a completed, unnotified run to be unviewed")
	}

	now := time.Now()
	r.NotifiedAt = &now
	if r.IsUnviewed() {
		t.Fatal("expected a notified run to not be unviewed")
	}
}

func TestRun_IsUnviewed_NonTerminalNeverCounts(t *testing.T) {
	r := &Run{Status: RunStatusRunning}
	if r.IsUnviewed() {
		t.Fatal("expected a non-terminal (running) run to never count as unviewed")
	}
}
