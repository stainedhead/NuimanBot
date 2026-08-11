package domain

import "time"

// RetentionPolicy describes how long a resource (Chat, Project, or History
// run) is kept before automatic deletion. A nil Period means "Never" — no
// automatic expiry (FR-003/014/023/043).
type RetentionPolicy struct {
	Period *time.Duration
}

// NeverExpire returns a RetentionPolicy representing "Never" (no auto-expiry).
func NeverExpire() RetentionPolicy {
	return RetentionPolicy{Period: nil}
}

// NewRetentionPolicy returns a RetentionPolicy that expires a resource after
// d has elapsed since its last activity. A non-positive d is treated as
// "Never" rather than "expire immediately" — a misconfigured zero/negative
// duration must never cause silent, unintended data loss.
func NewRetentionPolicy(d time.Duration) RetentionPolicy {
	if d <= 0 {
		return NeverExpire()
	}
	return RetentionPolicy{Period: &d}
}

// IsNever reports whether this policy represents "Never" (no auto-expiry).
func (p RetentionPolicy) IsNever() bool {
	return p.Period == nil
}

// IsExpired reports whether a resource whose most recent activity was at
// lastActivity is expired under this policy, evaluated as of now. Always
// false for "Never". Per spec.md Edge Case #12, callers must pass the
// resource's last-activity time (e.g. Chat's UpdatedAt), not its creation
// time, so an old-but-active resource is never treated as expired.
func (p RetentionPolicy) IsExpired(lastActivity, now time.Time) bool {
	if p.IsNever() {
		return false
	}
	return now.Sub(lastActivity) >= *p.Period
}
