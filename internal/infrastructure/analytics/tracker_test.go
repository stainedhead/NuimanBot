package analytics

import (
	"context"
	"testing"
	"time"
)

// Test: Initialize analytics
func TestInitialize(t *testing.T) {
	config := Config{
		Enabled:       true,
		ServiceName:   "nuimanbot-test",
		FlushInterval: 5 * time.Second,
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err = Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// Test: Track event
func TestTrackEvent(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Track an event
	TrackEvent(context.Background(), Event{
		Name:   "message_sent",
		UserID: "user-123",
		Properties: map[string]any{
			"platform": "cli",
			"length":   100,
		},
	})
}

// Test: Track user activity
func TestTrackUserActivity(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	ctx := context.Background()

	// Track various user activities
	TrackEvent(ctx, Event{
		Name:   "user_login",
		UserID: "user-123",
	})

	TrackEvent(ctx, Event{
		Name:   "message_sent",
		UserID: "user-123",
		Properties: map[string]any{
			"message_length": 50,
		},
	})

	TrackEvent(ctx, Event{
		Name:   "skill_executed",
		UserID: "user-123",
		Properties: map[string]any{
			"skill": "calculator",
		},
	})
}

// Test: Track metrics
func TestTrackMetric(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	ctx := context.Background()

	// Track various metrics
	TrackMetric(ctx, Metric{
		Name:  "response_time_ms",
		Value: 150.5,
		Tags: map[string]string{
			"provider": "anthropic",
			"model":    "claude-3-sonnet",
		},
	})

	TrackMetric(ctx, Metric{
		Name:  "token_usage",
		Value: 1024,
		Tags: map[string]string{
			"type": "completion",
		},
	})
}

// Test: Get statistics
func TestGetStatistics(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	ctx := context.Background()

	// Track some events
	for i := 0; i < 10; i++ {
		TrackEvent(ctx, Event{
			Name:   "test_event",
			UserID: "user-123",
		})
	}

	// Get statistics
	stats := GetStatistics(ctx)
	if stats == nil {
		t.Fatal("Expected non-nil statistics")
	}

	if stats.TotalEvents < 10 {
		t.Errorf("Expected at least 10 events, got %d", stats.TotalEvents)
	}
}

// Test: Disabled analytics
func TestDisabledAnalytics(t *testing.T) {
	config := Config{
		Enabled:     false,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should be noop
	TrackEvent(context.Background(), Event{
		Name:   "test_event",
		UserID: "user-123",
	})

	TrackMetric(context.Background(), Metric{
		Name:  "test_metric",
		Value: 100,
	})
}

// Test: Event with timestamp
func TestEventWithTimestamp(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	now := time.Now()
	TrackEvent(context.Background(), Event{
		Name:      "timestamped_event",
		UserID:    "user-123",
		Timestamp: now,
	})
}

// Test: Event batching
func TestEventBatching(t *testing.T) {
	config := Config{
		Enabled:       true,
		ServiceName:   "nuimanbot-test",
		FlushInterval: 1 * time.Second,
		BatchSize:     100,
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	ctx := context.Background()

	// Send multiple events
	for i := 0; i < 50; i++ {
		TrackEvent(ctx, Event{
			Name:   "batch_event",
			UserID: "user-123",
		})
	}

	// Allow time for batching
	time.Sleep(100 * time.Millisecond)

	stats := GetStatistics(ctx)
	if stats.TotalEvents < 50 {
		t.Errorf("Expected at least 50 events, got %d", stats.TotalEvents)
	}
}
