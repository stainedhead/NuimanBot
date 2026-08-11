package domain

import (
	"testing"
	"time"
)

func TestNeverExpire_IsNever(t *testing.T) {
	p := NeverExpire()
	if !p.IsNever() {
		t.Fatal("expected NeverExpire() to report IsNever() == true")
	}
}

func TestNewRetentionPolicy_PositiveDuration(t *testing.T) {
	p := NewRetentionPolicy(24 * time.Hour)
	if p.IsNever() {
		t.Fatal("expected a positive duration to not be Never")
	}
	if p.Period == nil || *p.Period != 24*time.Hour {
		t.Fatalf("expected Period == 24h, got %v", p.Period)
	}
}

func TestNewRetentionPolicy_NonPositiveDurationTreatedAsNever(t *testing.T) {
	cases := []time.Duration{0, -1 * time.Hour}
	for _, d := range cases {
		p := NewRetentionPolicy(d)
		if !p.IsNever() {
			t.Fatalf("expected duration %v to be treated as Never, got Period=%v", d, p.Period)
		}
	}
}

func TestRetentionPolicy_IsExpired_Never(t *testing.T) {
	p := NeverExpire()
	now := time.Now()
	longAgo := now.Add(-100 * 365 * 24 * time.Hour)
	if p.IsExpired(longAgo, now) {
		t.Fatal("Never policy must never report expired")
	}
}

func TestRetentionPolicy_IsExpired_BoundaryAndPastWindow(t *testing.T) {
	p := NewRetentionPolicy(90 * 24 * time.Hour)
	now := time.Now()

	notExpired := now.Add(-89 * 24 * time.Hour)
	if p.IsExpired(notExpired, now) {
		t.Fatal("expected activity within the retention window to not be expired")
	}

	exactlyAtBoundary := now.Add(-90 * 24 * time.Hour)
	if !p.IsExpired(exactlyAtBoundary, now) {
		t.Fatal("expected activity exactly at the retention window boundary to be expired (>=)")
	}

	pastWindow := now.Add(-91 * 24 * time.Hour)
	if !p.IsExpired(pastWindow, now) {
		t.Fatal("expected activity past the retention window to be expired")
	}
}

func TestRetentionPolicy_IsExpired_MeasuredFromLastActivityNotCreation(t *testing.T) {
	// Edge Case #12: an old-but-actively-used resource is never auto-deleted
	// mid-use. Simulated here by using a recent lastActivity even though the
	// (hypothetical) creation time would be far in the past.
	p := NewRetentionPolicy(24 * time.Hour)
	now := time.Now()
	recentActivity := now.Add(-1 * time.Hour)
	if p.IsExpired(recentActivity, now) {
		t.Fatal("expected recently-active resource to not be expired regardless of creation age")
	}
}
