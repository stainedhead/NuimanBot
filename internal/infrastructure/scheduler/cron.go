// Package scheduler provides the Job/Chore worker pool and cron-driven
// Chore scheduler (spec.md's largest net-new subsystem — see
// specs/260805-nuimanbot-extend-context-and-ui/architecture.md). It is the
// sole consumer of github.com/robfig/cron/v3 (research.md Q1): the domain
// layer stores a Schedule's raw CronExpression but never parses or
// evaluates it, keeping domain free of third-party dependencies.
package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser parses standard 5-field cron expressions (minute hour
// day-of-month month day-of-week) — the format produced by
// domain.NewScheduleFromPreset/NewScheduleFromCron and offered in the
// Chore schedule UI (FR-034).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ValidateCronExpression reports whether expr is a syntactically valid
// 5-field cron expression, without computing a next-fire time. Used to
// validate the "advanced raw cron expression field" (FR-034) at Chore
// create/edit time, independent of NextFireTime.
func ValidateCronExpression(expr string) error {
	if _, err := cronParser.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
}

// NextFireTime returns the next time expr fires strictly after `after`.
// Returns an error if expr is not a valid cron expression.
func NextFireTime(expr string, after time.Time) (time.Time, error) {
	schedule, err := cronParser.Parse(expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return schedule.Next(after), nil
}
