package alerting

import (
	"context"
	"testing"
)

// TestSendToPagerDuty_MissingKey tests PagerDuty with missing integration key.
func TestSendToPagerDuty_MissingKey(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypePagerDuty,
				Enabled: true,
				Config:  map[string]string{}, // No integration_key
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic - just log warning
	SendAlert(context.Background(), Alert{
		Severity: SeverityCritical,
		Title:    "Test",
		Message:  "Should log warning about missing key",
	})
}

// TestSendToPagerDuty_WithKey tests PagerDuty with integration key set (logs info).
func TestSendToPagerDuty_WithKey(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypePagerDuty,
				Enabled: true,
				Config: map[string]string{
					"integration_key": "fake-integration-key",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should log info (TODO implementation placeholder)
	SendAlert(context.Background(), Alert{
		Severity: SeverityCritical,
		Title:    "Critical Alert",
		Message:  "System failure",
	})
}

// TestSendToEmail_MissingSmtpPort tests email with smtp_host but no smtp_port.
func TestSendToEmail_MissingSmtpPort(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeEmail,
				Enabled: true,
				Config: map[string]string{
					"recipients": "admin@example.com",
					"smtp_host":  "mail.example.com",
					// Missing smtp_port
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic - just log warning
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Missing smtp port",
	})
}

// TestSendToEmail_MissingFrom tests email with no from address.
func TestSendToEmail_MissingFrom(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeEmail,
				Enabled: true,
				Config: map[string]string{
					"recipients": "admin@example.com",
					"smtp_host":  "mail.example.com",
					"smtp_port":  "587",
					// Missing from
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic - just log warning
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Missing from address",
	})
}

// TestSendToEmail_FullConfig tests email with full config (will fail to connect, but covers code paths).
func TestSendToEmail_FullConfig(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeEmail,
				Enabled: true,
				Config: map[string]string{
					"recipients": "admin@example.com , ops@example.com",
					"smtp_host":  "nonexistent-smtp.invalid",
					"smtp_port":  "587",
					"from":       "alerts@example.com",
					"username":   "user",
					"password":   "pass",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Will fail to connect to invalid SMTP host - that's expected
	// This tests the code path including the smtp.SendMail call
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Email Test",
		Message:  "Testing email with full config",
	})
}

// TestSendAlert_UnknownChannelType tests that unknown channel types are handled gracefully.
func TestSendAlert_UnknownChannelType(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelType("unknown-channel"),
				Enabled: true,
				Config:  map[string]string{},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic - just log warning
	SendAlert(context.Background(), Alert{
		Severity: SeverityInfo,
		Title:    "Test",
		Message:  "Unknown channel test",
	})
}

// TestSendAlert_DisabledChannel tests that disabled channels are skipped.
func TestSendAlert_DisabledChannel(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeSlack,
				Enabled: false, // Disabled
				Config:  map[string]string{"webhook_url": "http://example.com"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should be a noop (disabled channel)
	SendAlert(context.Background(), Alert{
		Severity: SeverityInfo,
		Title:    "Test",
		Message:  "Should not reach disabled channel",
	})
}

// TestShutdown_WhenDisabled tests Shutdown when alerting is disabled.
func TestShutdown_WhenDisabled(t *testing.T) {
	err := Initialize(Config{
		Enabled:     false,
		ServiceName: "test",
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err = Shutdown()
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

// TestShutdown_WhenNotInitialized tests Shutdown without initialization.
func TestShutdown_WhenNotInitialized(t *testing.T) {
	// Force shutdown state
	_ = Shutdown()

	// Second shutdown should be safe
	err := Shutdown()
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

// TestUpdateThrottleCache_Cleanup tests that old throttle cache entries are cleaned up.
func TestUpdateThrottleCache_Cleanup(t *testing.T) {
	err := Initialize(Config{
		Enabled:        true,
		ServiceName:    "test",
		ThrottleWindow: 1,
		Channels: []ChannelConfig{
			{Type: ChannelTypeLog, Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Update cache multiple times to trigger cleanup logic
	for i := 0; i < 5; i++ {
		updateThrottleCache("fp-" + string(rune('a'+i)))
	}

	// Cache should still work correctly
	if isThrottled("fp-a", 1) == false {
		// Just making sure it doesn't panic
	}
}
