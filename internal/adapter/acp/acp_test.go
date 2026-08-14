package acp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"nuimanbot/internal/adapter/acp"
	"nuimanbot/internal/domain"
)

// makePipe/mustWrite/closePipe let TestSessionCancelInterruptsInFlightPrompt
// drive acp.Server.Run's input incrementally (via an io.Pipe) instead of
// handing it a single static buffer — necessary to deterministically
// interleave a session/cancel notification with an in-flight session/prompt
// call.
func makePipe() (*io.PipeReader, *io.PipeWriter) {
	return io.Pipe()
}

func mustWrite(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("write to pipe: %v", err)
	}
}

func closePipe(w *io.PipeWriter) {
	_ = w.Close()
}

// fakeChatService is a test double standing in for chat.Service.
// ProcessMessage's real dependency (LLM, tools, memory) — the ACP server
// only needs to know it drives ChatService.ProcessMessage correctly.
type fakeChatService struct {
	mu      sync.Mutex
	lastMsg *domain.IncomingMessage
	reply   domain.OutgoingMessage
	err     error

	block     bool          // if true, ProcessMessage blocks until ctx.Done()
	started   chan struct{} // closed once ProcessMessage begins (for cancel synchronization)
	cancelled bool
}

func (f *fakeChatService) ProcessMessage(ctx context.Context, msg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	f.mu.Lock()
	f.lastMsg = msg
	f.mu.Unlock()

	if f.started != nil {
		close(f.started)
	}

	if f.block {
		<-ctx.Done()
		f.mu.Lock()
		f.cancelled = true
		f.mu.Unlock()
		return domain.OutgoingMessage{}, ctx.Err()
	}

	if f.err != nil {
		return domain.OutgoingMessage{}, f.err
	}
	return f.reply, nil
}

func (f *fakeChatService) LastMsg() *domain.IncomingMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastMsg
}

// lineWriter delivers each Write call's bytes as one channel item, letting
// tests wait for a specific output line without polling — the acp.Server
// always issues one Write call per JSON-RPC frame (see write in acp.go).
type lineWriter struct {
	lines chan []byte
}

func newLineWriter() *lineWriter {
	return &lineWriter{lines: make(chan []byte, 16)}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	b := make([]byte, len(p))
	copy(b, p)
	w.lines <- b
	return len(p), nil
}

func (w *lineWriter) next(t *testing.T) map[string]any {
	t.Helper()
	select {
	case b := <-w.lines:
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("output line is not valid JSON: %v\nline: %s", err, b)
		}
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for output line")
		return nil
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestInitialize(t *testing.T) {
	chat := &fakeChatService{}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")

	if err := server.Run(context.Background(), in, out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	resp := out.next(t)
	if resp["id"] != float64(1) {
		t.Errorf("expected id 1 echoed back, got %v", resp["id"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %v", resp)
	}
	agentInfo, ok := result["agentInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected agentInfo object, got %v", result)
	}
	if agentInfo["name"] != "nuimanbot" {
		t.Errorf("expected agentInfo.name = nuimanbot, got %v", agentInfo["name"])
	}
}

func TestSessionNewAndPrompt(t *testing.T) {
	chat := &fakeChatService{reply: domain.OutgoingMessage{Content: "hello from nuimanbot"}}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()

	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{}}`,
	}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")

	go func() {
		if err := server.Run(context.Background(), in, out); err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	}()

	newResp := out.next(t)
	result := newResp["result"].(map[string]any)
	sessionID, _ := result["sessionId"].(string)
	if sessionID == "" {
		t.Fatalf("expected non-empty sessionId, got %v", newResp)
	}

	// Second Run call reusing the same server instance/session map, driving
	// session/prompt against the session just created.
	promptLine := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":2,"method":"session/prompt","params":{"sessionId":%q,"prompt":[{"type":"text","text":"hi there"}]}}`,
		sessionID,
	)
	in2 := strings.NewReader(promptLine + "\n")
	if err := server.Run(context.Background(), in2, out); err != nil {
		t.Fatalf("Run (prompt) returned error: %v", err)
	}

	update := out.next(t)
	if update["method"] != "session/update" {
		t.Fatalf("expected a session/update notification first, got %v", update)
	}
	updateParams := update["params"].(map[string]any)
	if updateParams["sessionId"] != sessionID {
		t.Errorf("expected session/update to reference sessionId %q, got %v", sessionID, updateParams["sessionId"])
	}
	upd := updateParams["update"].(map[string]any)
	content := upd["content"].(map[string]any)
	if content["text"] != "hello from nuimanbot" {
		t.Errorf("expected update content text %q, got %v", "hello from nuimanbot", content["text"])
	}

	promptResp := out.next(t)
	if promptResp["id"] != float64(2) {
		t.Fatalf("expected id 2 echoed back, got %v", promptResp)
	}
	promptResult := promptResp["result"].(map[string]any)
	if promptResult["stopReason"] != "end_turn" {
		t.Errorf("expected stopReason end_turn, got %v", promptResult["stopReason"])
	}

	lastMsg := chat.LastMsg()
	if lastMsg == nil {
		t.Fatal("expected ChatService.ProcessMessage to have been called")
	}
	if lastMsg.Platform != domain.PlatformACP {
		t.Errorf("expected Platform=acp, got %v", lastMsg.Platform)
	}
	if lastMsg.Text != "hi there" {
		t.Errorf("expected Text=%q, got %q", "hi there", lastMsg.Text)
	}
}

func TestSessionPromptUnknownSession(t *testing.T) {
	chat := &fakeChatService{}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"does-not-exist","prompt":[{"type":"text","text":"hi"}]}}` + "\n")

	if err := server.Run(context.Background(), in, out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	resp := out.next(t)
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an error response for unknown session, got %v", resp)
	}
	if errObj["code"] != float64(-32602) {
		t.Errorf("expected invalid-params error code -32602, got %v", errObj["code"])
	}
}

func TestUnknownMethod(t *testing.T) {
	chat := &fakeChatService{}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"totally/unknown"}` + "\n")

	if err := server.Run(context.Background(), in, out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	resp := out.next(t)
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an error response for unknown method, got %v", resp)
	}
	if errObj["code"] != float64(-32601) {
		t.Errorf("expected method-not-found error code -32601, got %v", errObj["code"])
	}
}

func TestMalformedJSONLineDoesNotStopProcessing(t *testing.T) {
	chat := &fakeChatService{}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()
	in := strings.NewReader("not json at all\n" + `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")

	if err := server.Run(context.Background(), in, out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Two lines in, two responses out: a parse error for the first, then a
	// normal initialize result for the second — the malformed line must not
	// abort processing of subsequent valid lines.
	first := out.next(t)
	second := out.next(t)

	all := []map[string]any{first, second}
	sawParseError := false
	sawInitResult := false
	for _, r := range all {
		if errObj, ok := r["error"].(map[string]any); ok && errObj["code"] == float64(-32700) {
			sawParseError = true
		}
		if result, ok := r["result"].(map[string]any); ok {
			if _, ok := result["agentInfo"]; ok {
				sawInitResult = true
			}
		}
	}
	if !sawParseError {
		t.Errorf("expected a parse-error response among %v", all)
	}
	if !sawInitResult {
		t.Errorf("expected a successful initialize result among %v", all)
	}
}

func TestSessionNotificationsGetNoResponse(t *testing.T) {
	chat := &fakeChatService{}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()
	// No "id" field: a notification. Followed by a real request so we have
	// something deterministic to wait for — if the notification incorrectly
	// produced a response, it would arrive first and fail this assertion.
	in := strings.NewReader(
		`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"nope"}}` + "\n" +
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n",
	)

	if err := server.Run(context.Background(), in, out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	resp := out.next(t)
	if resp["id"] != float64(1) {
		t.Fatalf("expected the only response to be for id 1 (the notification should be silent), got %v", resp)
	}
}

func TestSessionCancelInterruptsInFlightPrompt(t *testing.T) {
	chat := &fakeChatService{block: true, started: make(chan struct{})}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()

	pr, pw := makePipe()
	runDone := make(chan error, 1)
	go func() { runDone <- server.Run(context.Background(), pr, out) }()

	mustWrite(t, pw, `{"jsonrpc":"2.0","id":1,"method":"session/new","params":{}}`+"\n")
	newResp := out.next(t)
	sessionID := newResp["result"].(map[string]any)["sessionId"].(string)

	mustWrite(t, pw, fmt.Sprintf(
		`{"jsonrpc":"2.0","id":2,"method":"session/prompt","params":{"sessionId":%q,"prompt":[{"type":"text","text":"hi"}]}}`+"\n",
		sessionID,
	))

	select {
	case <-chat.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ProcessMessage to start")
	}

	mustWrite(t, pw, fmt.Sprintf(`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":%q}}`+"\n", sessionID))
	closePipe(pw)

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to finish after cancel")
	}

	promptResp := out.next(t)
	result, ok := promptResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected a result (not error) for the cancelled prompt, got %v", promptResp)
	}
	if result["stopReason"] != "cancelled" {
		t.Errorf("expected stopReason=cancelled, got %v", result["stopReason"])
	}

	chat.mu.Lock()
	defer chat.mu.Unlock()
	if !chat.cancelled {
		t.Error("expected fakeChatService to have observed context cancellation")
	}
}

// TestSessionPrompt_ExtractsHistoryTextFromBuzzBundledPrompt guards a real
// production bug: saving the full Buzz-bundled prompt text as conversation
// history caused a later turn in the same session to see an EARLIER turn's
// entire bundle -- including that turn's own "[Buzz event]" trigger
// description -- replayed back as if still unanswered. The incoming
// domain.IncomingMessage's HistoryText must carry just the literal human
// message (the "Content:" line), for chat.Service.saveTurnMessages to
// persist instead of the full bundle. Text itself must stay the full
// bundle unchanged -- that's what actually gets sent to the LLM this turn.
func TestSessionPrompt_ExtractsHistoryTextFromBuzzBundledPrompt(t *testing.T) {
	chat := &fakeChatService{reply: domain.OutgoingMessage{Content: "ok"}}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{}}` + "\n")
	go func() {
		if err := server.Run(context.Background(), in, out); err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	}()
	newResp := out.next(t)
	sessionID := newResp["result"].(map[string]any)["sessionId"].(string)

	bundledText := "[Base]\nYou are operating inside the Buzz platform...\n" +
		"[Context]\nScope: dm\nChannel: DM (#abc)\n" +
		"[Buzz event: @mention]\n" +
		"Event ID: deadbeef\n" +
		"Channel: DM (#abc)\n" +
		"Kind: 9\n" +
		"From: stainedhead (npub: npub1..., hex: 38cf...)\n" +
		"Time: 2026-08-14T12:21:40+00:00\n" +
		"Content: @Iman good morning, do you know Pi?\n" +
		"Tags: [[\"h\",\"abc\"],[\"p\",\"iman\"]]\n" +
		"Parsed: mentions=[Iman (iman)]"

	payload := map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "session/prompt",
		"params": map[string]any{
			"sessionId": sessionID,
			"prompt":    []map[string]any{{"type": "text", "text": bundledText}},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	in2 := strings.NewReader(string(b) + "\n")
	if err := server.Run(context.Background(), in2, out); err != nil {
		t.Fatalf("Run (prompt) returned error: %v", err)
	}
	out.next(t) // session/update notification
	out.next(t) // session/prompt result

	lastMsg := chat.LastMsg()
	if lastMsg == nil {
		t.Fatal("expected ChatService.ProcessMessage to have been called")
	}
	if lastMsg.Text != bundledText {
		t.Errorf("expected Text to remain the full bundle unchanged, got %q", lastMsg.Text)
	}
	wantHistory := "@Iman good morning, do you know Pi?"
	if lastMsg.HistoryText != wantHistory {
		t.Errorf("expected HistoryText=%q, got %q", wantHistory, lastMsg.HistoryText)
	}
}

// TestSessionPrompt_NoBuzzEventBlockLeavesHistoryTextEmpty ensures a plain,
// non-Buzz-bundled prompt (e.g. a direct ACP test, or a future host with a
// different format) leaves HistoryText empty rather than extracting
// something wrong -- chat.Service.saveTurnMessages falls back to the full
// Text in that case, matching this test's existing sibling
// TestSessionNewAndPrompt.
func TestSessionPrompt_NoBuzzEventBlockLeavesHistoryTextEmpty(t *testing.T) {
	chat := &fakeChatService{reply: domain.OutgoingMessage{Content: "hello from nuimanbot"}}
	server := acp.NewServer(chat, testLogger())
	out := newLineWriter()

	lines := []string{`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{}}`}
	in := strings.NewReader(strings.Join(lines, "\n") + "\n")
	go func() {
		if err := server.Run(context.Background(), in, out); err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	}()
	newResp := out.next(t)
	sessionID := newResp["result"].(map[string]any)["sessionId"].(string)

	promptLine := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":2,"method":"session/prompt","params":{"sessionId":%q,"prompt":[{"type":"text","text":"hi there"}]}}`,
		sessionID,
	)
	in2 := strings.NewReader(promptLine + "\n")
	if err := server.Run(context.Background(), in2, out); err != nil {
		t.Fatalf("Run (prompt) returned error: %v", err)
	}
	out.next(t)
	out.next(t)

	lastMsg := chat.LastMsg()
	if lastMsg == nil {
		t.Fatal("expected ChatService.ProcessMessage to have been called")
	}
	if lastMsg.HistoryText != "" {
		t.Errorf("expected empty HistoryText for a non-Buzz-bundled prompt, got %q", lastMsg.HistoryText)
	}
}
