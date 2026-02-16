package alerting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// Test: Slack webhook sends correct payload to test server
func TestSlackWebhookIntegration(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}

		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeSlack,
				Enabled: true,
				Config: map[string]string{
					"webhook_url": server.URL,
					"channel":     "#test-alerts",
					"username":    "TestBot",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	SendAlert(context.Background(), Alert{
		Severity: SeverityCritical,
		Title:    "Test Critical Alert",
		Message:  "Something went wrong",
		Tags:     map[string]string{"component": "test"},
	})

	if len(receivedBody) == 0 {
		t.Fatal("No request body received by Slack test server")
	}

	var payload map[string]any
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("Failed to parse Slack payload: %v", err)
	}

	text, ok := payload["text"].(string)
	if !ok || !strings.Contains(text, "CRITICAL") {
		t.Errorf("Expected text to contain CRITICAL, got %q", text)
	}

	if channel, ok := payload["channel"].(string); !ok || channel != "#test-alerts" {
		t.Errorf("Expected channel #test-alerts, got %q", channel)
	}

	if username, ok := payload["username"].(string); !ok || username != "TestBot" {
		t.Errorf("Expected username TestBot, got %q", username)
	}
}

// Test: Slack payload construction
func TestBuildSlackPayload(t *testing.T) {
	alert := Alert{
		Severity: SeverityError,
		Title:    "DB Error",
		Message:  "Connection failed",
		Tags:     map[string]string{"env": "prod"},
	}
	config := map[string]string{
		"channel":  "#alerts",
		"username": "Bot",
	}

	payload := buildSlackPayload(alert, config)

	text, ok := payload["text"].(string)
	if !ok || !strings.Contains(text, "ERROR") || !strings.Contains(text, "DB Error") {
		t.Errorf("Unexpected text: %q", text)
	}

	if payload["channel"] != "#alerts" {
		t.Errorf("Expected channel #alerts, got %v", payload["channel"])
	}
	if payload["username"] != "Bot" {
		t.Errorf("Expected username Bot, got %v", payload["username"])
	}

	attachments, ok := payload["attachments"].([]map[string]any)
	if !ok || len(attachments) == 0 {
		t.Fatal("Expected attachments in payload")
	}

	if attachments[0]["color"] != "#CC0000" {
		t.Errorf("Expected error color #CC0000, got %v", attachments[0]["color"])
	}
}

// Test: Severity color mapping
func TestSeverityColor(t *testing.T) {
	tests := []struct {
		severity Severity
		want     string
	}{
		{SeverityCritical, "#FF0000"},
		{SeverityError, "#CC0000"},
		{SeverityWarning, "#FFA500"},
		{SeverityInfo, "#36A64F"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := severityColor(tt.severity)
			if got != tt.want {
				t.Errorf("severityColor(%s) = %s, want %s", tt.severity, got, tt.want)
			}
		})
	}
}

// Test: Email body construction
func TestBuildEmailBody(t *testing.T) {
	alert := Alert{
		Severity: SeverityCritical,
		Title:    "System Down",
		Message:  "Main service unreachable",
		Tags:     map[string]string{"component": "api"},
		Details:  map[string]any{"retry_count": 3},
	}

	body := buildEmailBody(alert, "alerts@example.com", []string{"admin@example.com", "ops@example.com"})

	if !strings.Contains(body, "From: alerts@example.com") {
		t.Error("Expected From header in email body")
	}
	if !strings.Contains(body, "To: admin@example.com, ops@example.com") {
		t.Error("Expected To header in email body")
	}
	if !strings.Contains(body, "Subject: [CRITICAL] System Down") {
		t.Error("Expected Subject header with severity and title")
	}
	if !strings.Contains(body, "Severity: critical") {
		t.Error("Expected severity in email body")
	}
	if !strings.Contains(body, "Message: Main service unreachable") {
		t.Error("Expected message in email body")
	}
	if !strings.Contains(body, "component: api") {
		t.Error("Expected tags in email body")
	}
	if !strings.Contains(body, "retry_count: 3") {
		t.Error("Expected details in email body")
	}
}

// Test: Slack alert with missing webhook URL is gracefully handled
func TestSlackMissingWebhookURL(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeSlack,
				Enabled: true,
				Config:  map[string]string{}, // No webhook_url
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Should log warning",
	})
}

// Test: Email alert with missing config is gracefully handled
func TestEmailMissingConfig(t *testing.T) {
	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeEmail,
				Enabled: true,
				Config:  map[string]string{}, // No config
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Should log warning",
	})
}

// Test: Slack webhook handles non-OK response
func TestSlackWebhookNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := Initialize(Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeSlack,
				Enabled: true,
				Config: map[string]string{
					"webhook_url": server.URL,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic, logs error
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Server returns 500",
	})
}

// Test: Fingerprint generation is consistent
func TestGenerateAlertFingerprint(t *testing.T) {
	alert := Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Msg",
	}

	fp1 := generateAlertFingerprint(alert)
	fp2 := generateAlertFingerprint(alert)

	if fp1 != fp2 {
		t.Error("Same alert should produce same fingerprint")
	}

	different := Alert{
		Severity: SeverityWarning,
		Title:    "Different",
		Message:  "Other",
	}
	fp3 := generateAlertFingerprint(different)

	if fp1 == fp3 {
		t.Error("Different alerts should produce different fingerprints")
	}
}

// Test: Throttle cache behavior
func TestThrottleCacheBehavior(t *testing.T) {
	err := Initialize(Config{
		Enabled:        true,
		ServiceName:    "nuimanbot-test",
		ThrottleWindow: 60,
		Channels: []ChannelConfig{
			{Type: ChannelTypeLog, Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer func() { _ = Shutdown() }()

	fp := "test-fingerprint"

	// Not throttled initially
	if isThrottled(fp, 60) {
		t.Error("Should not be throttled initially")
	}

	// Update cache
	updateThrottleCache(fp)

	// Now throttled
	if !isThrottled(fp, 60) {
		t.Error("Should be throttled after update")
	}

	// Different fingerprint not throttled
	if isThrottled("other-fingerprint", 60) {
		t.Error("Different fingerprint should not be throttled")
	}
}
