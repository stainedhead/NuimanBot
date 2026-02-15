package memoryv2

import (
	"testing"
	"time"
)

func TestIsSlowFTSQuery(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     bool
	}{
		{"under threshold", 10 * time.Millisecond, false},
		{"at threshold", FTSSlowQueryThreshold, false},
		{"over threshold", 51 * time.Millisecond, true},
		{"well over threshold", 200 * time.Millisecond, true},
		{"zero duration", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlowFTSQuery(tt.duration); got != tt.want {
				t.Errorf("isSlowFTSQuery(%v) = %v, want %v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestIsSlowRecall(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     bool
	}{
		{"under threshold", 50 * time.Millisecond, false},
		{"at threshold", RecallSlowThreshold, false},
		{"over threshold", 101 * time.Millisecond, true},
		{"well over threshold", 500 * time.Millisecond, true},
		{"zero duration", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlowRecall(tt.duration); got != tt.want {
				t.Errorf("isSlowRecall(%v) = %v, want %v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestIsSlowExtraction(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     bool
	}{
		{"under threshold", 1 * time.Second, false},
		{"at threshold", ExtractionSlowThreshold, false},
		{"over threshold", 6 * time.Second, true},
		{"well over threshold", 30 * time.Second, true},
		{"zero duration", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlowExtraction(tt.duration); got != tt.want {
				t.Errorf("isSlowExtraction(%v) = %v, want %v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestIsSlowConsolidation(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     bool
	}{
		{"under threshold", 2 * time.Second, false},
		{"at threshold", ConsolidationSlowThreshold, false},
		{"over threshold", 6 * time.Second, true},
		{"well over threshold", 30 * time.Second, true},
		{"zero duration", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSlowConsolidation(tt.duration); got != tt.want {
				t.Errorf("isSlowConsolidation(%v) = %v, want %v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestThresholdValues(t *testing.T) {
	// Verify threshold constants are reasonable
	if FTSSlowQueryThreshold != 50*time.Millisecond {
		t.Errorf("FTSSlowQueryThreshold = %v, want 50ms", FTSSlowQueryThreshold)
	}
	if RecallSlowThreshold != 100*time.Millisecond {
		t.Errorf("RecallSlowThreshold = %v, want 100ms", RecallSlowThreshold)
	}
	if ExtractionSlowThreshold != 5*time.Second {
		t.Errorf("ExtractionSlowThreshold = %v, want 5s", ExtractionSlowThreshold)
	}
	if ConsolidationSlowThreshold != 5*time.Second {
		t.Errorf("ConsolidationSlowThreshold = %v, want 5s", ConsolidationSlowThreshold)
	}

	// FTS should be the tightest threshold
	if FTSSlowQueryThreshold >= RecallSlowThreshold {
		t.Error("FTSSlowQueryThreshold should be less than RecallSlowThreshold")
	}
	if RecallSlowThreshold >= ExtractionSlowThreshold {
		t.Error("RecallSlowThreshold should be less than ExtractionSlowThreshold")
	}
}
