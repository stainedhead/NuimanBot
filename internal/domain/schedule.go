package domain

import "fmt"

// SchedulePreset names a common cron-schedule shorthand offered in the
// Chore schedule UI alongside the raw cron expression field (FR-034).
type SchedulePreset string

const (
	// SchedulePresetHourly fires once per hour, on the hour.
	SchedulePresetHourly SchedulePreset = "hourly"
	// SchedulePresetDaily fires once per day at midnight.
	SchedulePresetDaily SchedulePreset = "daily"
	// SchedulePresetWeekly fires once per week, Sunday at midnight.
	SchedulePresetWeekly SchedulePreset = "weekly"
	// SchedulePresetMonthly fires once per month, on the 1st at midnight.
	SchedulePresetMonthly SchedulePreset = "monthly"
)

// presetCronExpressions maps each SchedulePreset to its equivalent raw
// 5-field cron expression (minute hour day-of-month month day-of-week).
var presetCronExpressions = map[SchedulePreset]string{
	SchedulePresetHourly:  "0 * * * *",
	SchedulePresetDaily:   "0 0 * * *",
	SchedulePresetWeekly:  "0 0 * * 0",
	SchedulePresetMonthly: "0 0 1 * *",
}

// Schedule is a Chore's cron-style recurrence (FR-032/FR-034/FR-033).
// CronExpression is always the authoritative field used for evaluation;
// Preset, when non-nil, records which UI preset (if any) was used to derive
// CronExpression, purely for round-tripping the UI's selected control.
type Schedule struct {
	CronExpression string
	Preset         *SchedulePreset
}

// NewScheduleFromPreset returns a Schedule derived from a known preset.
// Returns ErrInvalidInput for an unrecognized preset.
func NewScheduleFromPreset(preset SchedulePreset) (Schedule, error) {
	expr, ok := presetCronExpressions[preset]
	if !ok {
		return Schedule{}, fmt.Errorf("%w: unknown schedule preset %q", ErrInvalidInput, preset)
	}
	p := preset
	return Schedule{CronExpression: expr, Preset: &p}, nil
}

// NewScheduleFromCron returns a Schedule from a raw cron expression (the
// "advanced raw cron expression field" of FR-034). Only non-emptiness is
// validated here; cron grammar validation is an infrastructure-layer
// concern (internal/infrastructure/scheduler, backed by robfig/cron/v3) so
// the domain layer stays free of that third-party dependency.
func NewScheduleFromCron(expr string) (Schedule, error) {
	if expr == "" {
		return Schedule{}, fmt.Errorf("%w: cron expression must not be empty", ErrInvalidInput)
	}
	return Schedule{CronExpression: expr}, nil
}

// KnownPresets returns the list of supported presets in a stable, UI-friendly
// order (coarsest to finest is intentionally avoided; hourly-to-monthly
// ascending granularity matches how the preset picker should be laid out).
func KnownPresets() []SchedulePreset {
	return []SchedulePreset{
		SchedulePresetHourly,
		SchedulePresetDaily,
		SchedulePresetWeekly,
		SchedulePresetMonthly,
	}
}
