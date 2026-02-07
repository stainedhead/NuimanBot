package alerting

import (
	"context"
	"testing"
)

// Test: Initialize alerter
func TestInitialize(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeLog,
				Enabled: true,
			},
		},
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

// Test: Send alert
func TestSendAlert(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeLog,
				Enabled: true,
			},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Send a critical alert
	SendAlert(context.Background(), Alert{
		Severity: SeverityCritical,
		Title:    "Test Alert",
		Message:  "This is a test alert",
		Tags:     map[string]string{"component": "test"},
	})
}

// Test: Different severity levels
func TestAlertSeverities(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeLog,
				Enabled: true,
			},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	severities := []Severity{
		SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityCritical,
	}

	for _, sev := range severities {
		SendAlert(context.Background(), Alert{
			Severity: sev,
			Title:    "Test Alert",
			Message:  "Testing severity: " + string(sev),
		})
	}
}

// Test: Throttling (same alert within threshold)
func TestAlertThrottling(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeLog,
				Enabled: true,
			},
		},
		ThrottleWindow: 60, // 60 seconds
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	alert := Alert{
		Severity: SeverityCritical,
		Title:    "Duplicate Alert",
		Message:  "This should be throttled",
	}

	// Send same alert twice
	SendAlert(context.Background(), alert)
	SendAlert(context.Background(), alert)

	// Both should succeed but second should be throttled (implementation detail)
}

// Test: Multiple channels
func TestMultipleChannels(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeLog,
				Enabled: true,
			},
			{
				Type:    ChannelTypeSlack,
				Enabled: false, // Disabled for testing
				Config: map[string]string{
					"webhook_url": "https://example.com/webhook",
				},
			},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	SendAlert(context.Background(), Alert{
		Severity: SeverityWarning,
		Title:    "Multi-channel Test",
		Message:  "Testing multiple channels",
	})
}

// Test: Disabled alerting
func TestDisabledAlerting(t *testing.T) {
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
	SendAlert(context.Background(), Alert{
		Severity: SeverityCritical,
		Title:    "Test Alert",
		Message:  "This should not be sent",
	})
}

// Test: Alert with details
func TestAlertWithDetails(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeLog,
				Enabled: true,
			},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Database Connection Failed",
		Message:  "Unable to connect to database",
		Tags: map[string]string{
			"component": "database",
			"host":      "db.example.com",
		},
		Details: map[string]any{
			"error":       "connection timeout",
			"retry_count": 3,
			"duration_ms": 5000,
		},
	})
}
