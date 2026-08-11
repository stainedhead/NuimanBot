package domain

import (
	"errors"
	"testing"
)

func TestNewScheduleFromPreset_KnownPresets(t *testing.T) {
	for _, preset := range KnownPresets() {
		s, err := NewScheduleFromPreset(preset)
		if err != nil {
			t.Fatalf("preset %q: unexpected error: %v", preset, err)
		}
		if s.CronExpression == "" {
			t.Fatalf("preset %q: expected a non-empty cron expression", preset)
		}
		if s.Preset == nil || *s.Preset != preset {
			t.Fatalf("preset %q: expected Preset to round-trip", preset)
		}
	}
}

func TestNewScheduleFromPreset_Unknown(t *testing.T) {
	_, err := NewScheduleFromPreset(SchedulePreset("fortnightly"))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewScheduleFromCron_Valid(t *testing.T) {
	s, err := NewScheduleFromCron("*/5 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.CronExpression != "*/5 * * * *" {
		t.Fatalf("expected cron expression to round-trip, got %q", s.CronExpression)
	}
	if s.Preset != nil {
		t.Fatal("expected Preset to be nil for a raw cron expression")
	}
}

func TestNewScheduleFromCron_Empty(t *testing.T) {
	_, err := NewScheduleFromCron("")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty expression, got %v", err)
	}
}

func TestKnownPresets_AllHaveMappings(t *testing.T) {
	presets := KnownPresets()
	if len(presets) == 0 {
		t.Fatal("expected at least one known preset")
	}
	seen := make(map[SchedulePreset]bool)
	for _, p := range presets {
		if seen[p] {
			t.Fatalf("duplicate preset in KnownPresets(): %q", p)
		}
		seen[p] = true
	}
}
