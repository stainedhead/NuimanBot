package slack

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/slack-go/slack"
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

// --- Part C / P5.7: pending-confirmation rendering (Block Kit buttons) ---

func TestPendingConfirmationID(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantID   string
		wantOK   bool
	}{
		{"nil metadata", nil, "", false},
		{"no status key", map[string]any{"confirmation_id": "abc"}, "", false},
		{"wrong status", map[string]any{"status": "confirmation_denied", "confirmation_id": "abc"}, "", false},
		{"pending but no id", map[string]any{"status": "pending_confirmation"}, "", false},
		{"pending with id", map[string]any{"status": "pending_confirmation", "confirmation_id": "abc-123"}, "abc-123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := pendingConfirmationID(tt.metadata)
			if ok != tt.wantOK || id != tt.wantID {
				t.Errorf("pendingConfirmationID(%v) = (%q, %v), want (%q, %v)", tt.metadata, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestBuildMessageOptions_PendingConfirmation_RendersApproveDenyButtons(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	msg := domain.OutgoingMessage{
		RecipientID: "C123",
		Content:     "About to run github.pr_merge on PR #42",
		Metadata: map[string]any{
			"status":          "pending_confirmation",
			"confirmation_id": "conf-abc",
		},
	}

	opts := gw.buildMessageOptions(msg)

	_, values, err := slack.UnsafeApplyMsgOptions("tok", "C123", "https://slack.com/api/", opts...)
	if err != nil {
		t.Fatalf("UnsafeApplyMsgOptions() error = %v", err)
	}

	fallbackText := values.Get("text")
	if !strings.Contains(fallbackText, "Confirmation required") {
		t.Errorf("fallback text = %q, want it to mention confirmation required", fallbackText)
	}

	blocksJSON := values.Get("blocks")
	if blocksJSON == "" {
		t.Fatal("expected blocks to be set for a pending confirmation message")
	}
	if !strings.Contains(blocksJSON, confirmActionApprove) {
		t.Errorf("blocks JSON missing approve action id %q: %s", confirmActionApprove, blocksJSON)
	}
	if !strings.Contains(blocksJSON, confirmActionDeny) {
		t.Errorf("blocks JSON missing deny action id %q: %s", confirmActionDeny, blocksJSON)
	}
	if !strings.Contains(blocksJSON, "conf-abc") {
		t.Errorf("blocks JSON missing confirmation id value: %s", blocksJSON)
	}
	if !strings.Contains(blocksJSON, msg.Content) {
		t.Errorf("blocks JSON missing original content: %s", blocksJSON)
	}
}

func TestBuildMessageOptions_Normal_PlainText(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	msg := domain.OutgoingMessage{
		RecipientID: "C123",
		Content:     "just a normal reply",
	}

	opts := gw.buildMessageOptions(msg)

	_, values, err := slack.UnsafeApplyMsgOptions("tok", "C123", "https://slack.com/api/", opts...)
	if err != nil {
		t.Fatalf("UnsafeApplyMsgOptions() error = %v", err)
	}

	if values.Get("text") != msg.Content {
		t.Errorf("text = %q, want %q", values.Get("text"), msg.Content)
	}
	if values.Get("blocks") != "" {
		t.Errorf("expected no blocks for a normal message, got %s", values.Get("blocks"))
	}
}

func TestBuildMessageOptions_PendingConfirmation_MissingID_FallsBackToPlainText(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	msg := domain.OutgoingMessage{
		RecipientID: "C123",
		Content:     "pending but no id",
		Metadata: map[string]any{
			"status": "pending_confirmation",
		},
	}

	opts := gw.buildMessageOptions(msg)
	_, values, err := slack.UnsafeApplyMsgOptions("tok", "C123", "https://slack.com/api/", opts...)
	if err != nil {
		t.Fatalf("UnsafeApplyMsgOptions() error = %v", err)
	}

	if values.Get("blocks") != "" {
		t.Errorf("expected no blocks when confirmation_id is missing, got %s", values.Get("blocks"))
	}
	if values.Get("text") != msg.Content {
		t.Errorf("text = %q, want plain content %q", values.Get("text"), msg.Content)
	}
}

func TestBuildMessageOptions_WithThreadTS(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	msg := domain.OutgoingMessage{
		RecipientID: "C123",
		Content:     "reply in thread",
		Metadata: map[string]any{
			"thread_ts": "1234.5678",
		},
	}

	opts := gw.buildMessageOptions(msg)
	_, values, err := slack.UnsafeApplyMsgOptions("tok", "C123", "https://slack.com/api/", opts...)
	if err != nil {
		t.Fatalf("UnsafeApplyMsgOptions() error = %v", err)
	}
	if values.Get("thread_ts") != "1234.5678" {
		t.Errorf("thread_ts = %q, want %q", values.Get("thread_ts"), "1234.5678")
	}
}

func TestConfirmationReplyText(t *testing.T) {
	tests := []struct {
		name       string
		actions    []*slack.BlockAction
		wantText   string
		wantOK     bool
		nilElement bool
	}{
		{"empty", nil, "", false, false},
		{"approve", []*slack.BlockAction{{ActionID: confirmActionApprove}}, "yes", true, false},
		{"deny", []*slack.BlockAction{{ActionID: confirmActionDeny}}, "no", true, false},
		{"unknown action id", []*slack.BlockAction{{ActionID: "something_else"}}, "", false, false},
		{"nil element skipped", []*slack.BlockAction{nil, {ActionID: confirmActionDeny}}, "no", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, ok := confirmationReplyText(tt.actions)
			if ok != tt.wantOK || text != tt.wantText {
				t.Errorf("confirmationReplyText() = (%q, %v), want (%q, %v)", text, ok, tt.wantText, tt.wantOK)
			}
		})
	}
}

func TestHandleInteraction_Approve(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	var received domain.IncomingMessage
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		received = msg
		return nil
	}

	callback := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		User: slack.User{ID: "U999"},
		Channel: slack.Channel{
			GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "C999"}},
		},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{{ActionID: confirmActionApprove, Value: "conf-1"}},
		},
	}

	gw.handleInteraction(context.Background(), callback)

	if received.Text != "yes" {
		t.Errorf("Text = %q, want %q", received.Text, "yes")
	}
	if received.PlatformUID != "U999" {
		t.Errorf("PlatformUID = %q, want %q", received.PlatformUID, "U999")
	}
	if received.Platform != domain.PlatformSlack {
		t.Errorf("Platform = %q, want %q", received.Platform, domain.PlatformSlack)
	}
}

func TestHandleInteraction_Deny(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	var received domain.IncomingMessage
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		received = msg
		return nil
	}

	callback := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		User: slack.User{ID: "U999"},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{{ActionID: confirmActionDeny, Value: "conf-1"}},
		},
	}

	gw.handleInteraction(context.Background(), callback)

	if received.Text != "no" {
		t.Errorf("Text = %q, want %q", received.Text, "no")
	}
}

func TestHandleInteraction_NonBlockActionsIgnored(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	callback := slack.InteractionCallback{
		Type: slack.InteractionTypeViewSubmission,
	}

	gw.handleInteraction(context.Background(), callback)

	if handlerCalled {
		t.Error("handler should not be called for a non-block_actions interaction")
	}
}

func TestHandleInteraction_NoHandler(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}
	// messageHandler is nil

	callback := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		User: slack.User{ID: "U999"},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{{ActionID: confirmActionApprove}},
		},
	}

	// Should not panic when no handler is registered
	gw.handleInteraction(context.Background(), callback)
}

func TestHandleInteraction_UnknownActionIgnored(t *testing.T) {
	gw := &Gateway{config: &config.SlackConfig{Enabled: true}}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	callback := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{{ActionID: "some_other_button"}},
		},
	}

	gw.handleInteraction(context.Background(), callback)

	if handlerCalled {
		t.Error("handler should not be called for an unrecognized action id")
	}
}
