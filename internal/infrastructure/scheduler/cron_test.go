package scheduler

import (
	"testing"
	"time"
)

func TestValidateCronExpression_Valid(t *testing.T) {
	exprs := []string{"0 0 * * *", "*/5 * * * *", "0 0 * * 0", "0 0 1 * *", "0 * * * *"}
	for _, e := range exprs {
		if err := ValidateCronExpression(e); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", e, err)
		}
	}
}

func TestValidateCronExpression_Invalid(t *testing.T) {
	exprs := []string{"", "not a cron", "99 99 * * *", "* * * *"}
	for _, e := range exprs {
		if err := ValidateCronExpression(e); err == nil {
			t.Errorf("expected %q to be invalid", e)
		}
	}
}

func TestNextFireTime_Daily(t *testing.T) {
	after := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	next, err := NextFireTime("0 0 * * *", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("expected %v, got %v", want, next)
	}
}

func TestNextFireTime_Hourly(t *testing.T) {
	after := time.Date(2026, 8, 5, 10, 15, 0, 0, time.UTC)
	next, err := NextFireTime("0 * * * *", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 5, 11, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("expected %v, got %v", want, next)
	}
}

func TestNextFireTime_InvalidExpression(t *testing.T) {
	_, err := NextFireTime("garbage", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}
