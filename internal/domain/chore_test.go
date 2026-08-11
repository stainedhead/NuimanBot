package domain

import (
	"testing"
	"time"
)

func TestChore_IsDue_ConfirmedAndDue(t *testing.T) {
	now := time.Now()
	c := &Chore{ScheduleConfirmed: true, NextFireTime: now.Add(-time.Minute)}
	if !c.IsDue(now) {
		t.Fatal("expected a confirmed chore whose fire time has passed to be due")
	}
}

func TestChore_IsDue_ExactlyAtFireTime(t *testing.T) {
	now := time.Now()
	c := &Chore{ScheduleConfirmed: true, NextFireTime: now}
	if !c.IsDue(now) {
		t.Fatal("expected a chore due exactly now to be due")
	}
}

func TestChore_IsDue_NotYetDue(t *testing.T) {
	now := time.Now()
	c := &Chore{ScheduleConfirmed: true, NextFireTime: now.Add(time.Hour)}
	if c.IsDue(now) {
		t.Fatal("expected a chore whose fire time is in the future to not be due")
	}
}

func TestChore_IsDue_UnconfirmedNeverFires(t *testing.T) {
	// Edge Case #6: an unconfirmed, agent-proposed schedule does not fire,
	// even if NextFireTime has technically passed.
	now := time.Now()
	c := &Chore{ScheduleConfirmed: false, NextFireTime: now.Add(-time.Hour)}
	if c.IsDue(now) {
		t.Fatal("expected an unconfirmed chore to never be due")
	}
}

func TestChore_IsDue_PendingDeletionNeverFires(t *testing.T) {
	now := time.Now()
	c := &Chore{ScheduleConfirmed: true, NextFireTime: now.Add(-time.Hour), PendingDeletion: true}
	if c.IsDue(now) {
		t.Fatal("expected a chore pending deletion to never be due")
	}
}
