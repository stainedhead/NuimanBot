package alerting

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSendToSlack_RequestError tests Slack webhook when the request fails.
func TestSendToSlack_RequestError(t *testing.T) {
	// Create a server that immediately closes the connection to simulate network error
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // Close immediately so connections fail

	initErr := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeSlack,
				Enabled: true,
				Config: map[string]string{
					"webhook_url": "http://" + addr,
				},
			},
		},
	})
	if initErr != nil {
		t.Fatalf("Initialize failed: %v", initErr)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic - logs error
	SendAlert(context.Background(), Alert{
		Severity: SeverityError,
		Title:    "Test",
		Message:  "Connection error",
	})
}

// TestUpdateThrottleCache_OldEntryCleanup tests cleanup of old throttle cache entries.
func TestUpdateThrottleCache_OldEntryCleanup(t *testing.T) {
	_ = Initialize(Config{
		Enabled:        true,
		ServiceName:    "test",
		ThrottleWindow: 1,
		Channels:       []ChannelConfig{{Type: ChannelTypeLog, Enabled: true}},
	})
	defer func() { _ = Shutdown() }()

	// Manually insert an old entry into the cache to trigger cleanup
	throttleMu.Lock()
	throttleCache["old-entry"] = time.Now().Add(-2 * time.Hour)
	throttleMu.Unlock()

	// Call updateThrottleCache to trigger cleanup
	updateThrottleCache("new-entry")

	// Verify old entry was cleaned up
	throttleMu.RLock()
	_, exists := throttleCache["old-entry"]
	throttleMu.RUnlock()

	if exists {
		t.Error("Expected old entry to be cleaned up")
	}
}

// TestSendToSlack_CreateRequestError tests Slack webhook with invalid URL format.
func TestSendToSlack_CreateRequestError(t *testing.T) {
	initErr := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
		Channels: []ChannelConfig{
			{
				Type:    ChannelTypeSlack,
				Enabled: true,
				Config: map[string]string{
					"webhook_url": "://invalid-url-format",
				},
			},
		},
	})
	if initErr != nil {
		t.Fatalf("Initialize failed: %v", initErr)
	}
	defer func() { _ = Shutdown() }()

	// Should not panic - logs error for invalid URL
	SendAlert(context.Background(), Alert{
		Severity: SeverityWarning,
		Title:    "Test",
		Message:  "Invalid URL",
	})
}

// TestSendAlert_NotInitialized tests SendAlert when alerting is not initialized.
func TestSendAlert_NotInitialized(t *testing.T) {
	// Ensure uninitialized state
	_ = Shutdown()

	// Should be a complete noop
	SendAlert(context.Background(), Alert{
		Severity: SeverityCritical,
		Title:    "Test",
		Message:  "Not initialized",
	})
}

// TestSlackWebhookHandlesFailureGracefully tests that Slack failures don't crash.
func TestSlackWebhookHandlesFailureGracefully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a 500 error
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	initErr := Initialize(Config{
		Enabled:     true,
		ServiceName: "test",
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
	if initErr != nil {
		t.Fatalf("Initialize failed: %v", initErr)
	}
	defer func() { _ = Shutdown() }()

	// Send multiple alerts
	for i := 0; i < 3; i++ {
		SendAlert(context.Background(), Alert{
			Severity: SeverityError,
			Title:    "Test Error",
			Message:  "Server failure",
		})
	}
}
