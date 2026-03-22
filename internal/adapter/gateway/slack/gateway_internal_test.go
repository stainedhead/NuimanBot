package slack

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		name string
		ts   string
	}{
		{
			name: "valid timestamp",
			ts:   "1609459200.000000",
		},
		{
			name: "empty timestamp returns now-ish",
			ts:   "",
		},
		{
			name: "invalid format returns now-ish",
			ts:   "not-a-timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSlackTimestamp(tt.ts)
			// Result should be a valid time (not zero)
			if result.IsZero() {
				t.Error("parseSlackTimestamp() returned zero time")
			}
		})
	}
}

func TestParseSlackTimestamp_CorrectValue(t *testing.T) {
	// 1609459200 = 2021-01-01 00:00:00 UTC
	ts := "1609459200.000000"
	result := parseSlackTimestamp(ts)
	expected := time.Unix(1609459200, 0)
	if !result.Equal(expected) {
		t.Errorf("parseSlackTimestamp(%q) = %v, want %v", ts, result, expected)
	}
}

func TestHandleMessage_EmptyText(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled: true,
	}
	gw := &Gateway{config: cfg}

	// Handler should not be called when text is empty
	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	gw.handleMessage(context.Background(), "U123", "", "C456", "ts.1", "", "app_mention")
	if handlerCalled {
		t.Error("handler should not be called for empty text")
	}
}

func TestHandleMessage_WithHandler(t *testing.T) {
	cfg := &config.SlackConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	var receivedMsg domain.IncomingMessage
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		receivedMsg = msg
		return nil
	}

	gw.handleMessage(context.Background(), "U123", "hello world", "C456", "1234567890.000000", "1234567890.000000", "app_mention")

	if receivedMsg.Text != "hello world" {
		t.Errorf("Text = %q, want %q", receivedMsg.Text, "hello world")
	}
	if receivedMsg.Platform != domain.PlatformSlack {
		t.Errorf("Platform = %q, want %q", receivedMsg.Platform, domain.PlatformSlack)
	}
	if receivedMsg.PlatformUID != "U123" {
		t.Errorf("PlatformUID = %q, want %q", receivedMsg.PlatformUID, "U123")
	}
	if receivedMsg.Metadata == nil {
		t.Error("Metadata should be set")
	}
	if receivedMsg.Metadata["channel"] != "C456" {
		t.Errorf("Metadata channel = %q, want %q", receivedMsg.Metadata["channel"], "C456")
	}
}

func TestHandleMessage_HandlerError(t *testing.T) {
	cfg := &config.SlackConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		return errors.New("handler error")
	}

	// Should not panic even when handler returns error
	gw.handleMessage(context.Background(), "U123", "hello", "C456", "ts.1", "", "direct_message")
}

func TestHandleMessage_NoHandler(t *testing.T) {
	cfg := &config.SlackConfig{Enabled: true}
	gw := &Gateway{config: cfg}
	// messageHandler is nil

	// Should not panic when no handler is registered
	gw.handleMessage(context.Background(), "U123", "hello", "C456", "ts.1", "", "app_mention")
}

func TestStop_WithCancel(t *testing.T) {
	cfg := &config.SlackConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	cancelCalled := false
	gw.cancel = func() {
		cancelCalled = true
	}

	if err := gw.Stop(context.Background()); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if !cancelCalled {
		t.Error("cancel should be called when stopping with active cancel func")
	}
}
