// Package acp implements a minimal Agent Client Protocol (ACP) agent:
// JSON-RPC 2.0 over newline-delimited JSON on stdio. It exists so NuimanBot
// can be registered as a custom Buzz "harness" — Buzz's buzz-acp bridge
// spawns one NuimanBot subprocess per conversation and drives it via this
// protocol, distinct from internal/adapter/gateway/buzz (which connects
// outward to a Nostr relay as a domain.Gateway instead).
//
// Implemented against the protocol shapes documented at
// agentclientprotocol.com and observed in Buzz's own buzz-acp `initialize`
// response (field names for agentCapabilities/agentInfo). NuimanBot has not
// yet been exercised against a live ACP host, so field names — particularly
// session/update's payload shape — may need adjustment once verified
// against a real Buzz session; see support_docs/buzz-acp-harness-guide.md's
// Verifying section.
package acp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nuimanbot/internal/domain"
)

const (
	jsonrpcVersion = "2.0"

	methodInitialize     = "initialize"
	methodSessionNew     = "session/new"
	methodSessionPrompt  = "session/prompt"
	methodSessionCancel  = "session/cancel"
	methodSessionUpdate  = "session/update" // agent -> client notification only, never dispatched inbound
	agentMessageChunkTag = "agent_message_chunk"

	errCodeParse          = -32700
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternal       = -32603

	acpProtocolVersion = 1

	// maxLineSize bounds a single JSON-RPC frame — generous enough for any
	// realistic chat prompt while still bounding worst-case memory use.
	maxLineSize = 10 * 1024 * 1024
)

// AgentName/AgentVersion identify this agent in initialize's response and
// are exported for reuse by cmd/nuimanbot's startup logging.
const (
	AgentName    = "nuimanbot"
	AgentVersion = "1.0.0"
)

// ChatService is the subset of chat.Service's API the ACP server drives.
// Declared here, at the point of use, per this project's interface
// convention (AGENTS.md).
type ChatService interface {
	ProcessMessage(ctx context.Context, msg *domain.IncomingMessage) (domain.OutgoingMessage, error)
}

// Server implements the ACP agent side of the protocol described in the
// package doc comment.
type Server struct {
	chat   ChatService
	logger *slog.Logger

	writeMu sync.Mutex
	out     io.Writer

	sessions sync.Map // sessionID string -> *session

	reqCounter uint64
}

// session tracks per-ACP-session state: the domain.IncomingMessage
// PlatformUID used to key conversation history/RBAC for this session (see
// chat.Service.resolveUser), and the cancel func for whichever
// session/prompt call is currently in flight for it, if any.
type session struct {
	platformUID string

	mu     sync.Mutex
	cancel context.CancelFunc
}

// NewServer constructs an ACP Server. logger receives all diagnostic
// output — the server never writes anything to the ACP transport itself
// (its Run's out parameter) except JSON-RPC frames. A nil logger falls back
// to slog.Default(); callers driving a real ACP session over stdio must
// pass a logger targeting stderr (see cmd/nuimanbot/acp.go), never the
// package-default stdout logger, or log lines will corrupt the protocol
// stream.
func NewServer(chat ChatService, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{chat: chat, logger: logger}
}

// Run reads newline-delimited JSON-RPC requests/notifications from in and
// writes responses/notifications to out until in reaches EOF or produces a
// read error. Each line is dispatched in its own goroutine — required so a
// session/cancel notification can interrupt a still-running session/prompt
// call for the same session; Run waits for every in-flight handler to
// finish before returning. Session state (created by session/new) persists
// on the Server across multiple sequential Run calls (a later call must
// not start until an earlier one has returned — the field backing out is
// guarded by writeMu, but nothing here serializes overlapping Run calls
// themselves), so a caller may drive Run repeatedly (e.g. once per batch of
// input) against the same Server.
func (s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	s.writeMu.Lock()
	s.out = out
	s.writeMu.Unlock()

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var wg sync.WaitGroup
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		lineCopy := append([]byte(nil), line...) // scanner reuses its buffer on the next Scan
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handleLine(ctx, lineCopy)
		}()
	}
	wg.Wait()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("acp: reading input: %w", err)
	}
	return nil
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func (s *Server) handleLine(ctx context.Context, line []byte) {
	var msg rpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		s.writeError(nil, errCodeParse, "parse error: "+err.Error())
		return
	}

	isNotification := len(msg.ID) == 0 || string(msg.ID) == "null"

	switch msg.Method {
	case methodInitialize:
		if isNotification {
			s.logger.Warn("acp: initialize sent as a notification (no id); ignoring, nothing to reply to")
			return
		}
		s.handleInitialize(msg)
	case methodSessionNew:
		if isNotification {
			s.logger.Warn("acp: session/new sent as a notification (no id); ignoring, nothing to reply to")
			return
		}
		s.handleSessionNew(msg)
	case methodSessionPrompt:
		if isNotification {
			s.logger.Warn("acp: session/prompt sent as a notification (no id); ignoring, nothing to reply to")
			return
		}
		s.handleSessionPrompt(ctx, msg)
	case methodSessionCancel:
		// Always fire-and-forget per the ACP spec, regardless of whether the
		// caller included an id.
		s.handleSessionCancel(msg)
	default:
		if isNotification {
			s.logger.Warn("acp: unknown notification method, ignoring", "method", msg.Method)
			return
		}
		s.writeError(msg.ID, errCodeMethodNotFound, "method not found: "+msg.Method)
	}
}

type agentCapabilities struct {
	LoadSession        bool               `json:"loadSession"`
	PromptCapabilities promptCapabilities `json:"promptCapabilities"`
}

type promptCapabilities struct {
	EmbeddedContext bool `json:"embeddedContext"`
	Image           bool `json:"image"`
}

type agentInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities agentCapabilities `json:"agentCapabilities"`
	AgentInfo         agentInfo         `json:"agentInfo"`
	AuthMethods       []any             `json:"authMethods"`
}

func (s *Server) handleInitialize(msg rpcMessage) {
	s.writeResult(msg.ID, initializeResult{
		ProtocolVersion: acpProtocolVersion,
		AgentCapabilities: agentCapabilities{
			// LoadSession/EmbeddedContext/Image are all false: NuimanBot
			// does not persist ACP sessions across process restarts, and
			// session/prompt content blocks other than plain text are not
			// yet consumed (see promptText).
			LoadSession: false,
			PromptCapabilities: promptCapabilities{
				EmbeddedContext: false,
				Image:           false,
			},
		},
		AgentInfo:   agentInfo{Name: AgentName, Version: AgentVersion},
		AuthMethods: []any{},
	})
}

type sessionNewResult struct {
	SessionID string `json:"sessionId"`
}

func (s *Server) handleSessionNew(msg rpcMessage) {
	id, err := newSessionID()
	if err != nil {
		s.writeError(msg.ID, errCodeInternal, "failed to create session: "+err.Error())
		return
	}
	s.sessions.Store(id, &session{platformUID: "acp-" + id})
	s.writeResult(msg.ID, sessionNewResult{SessionID: id})
}

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type sessionPromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []contentBlock `json:"prompt"`
}

type sessionPromptResult struct {
	StopReason string `json:"stopReason"`
}

type sessionUpdate struct {
	SessionUpdate string       `json:"sessionUpdate"`
	Content       contentBlock `json:"content"`
}

type sessionUpdateParams struct {
	SessionID string        `json:"sessionId"`
	Update    sessionUpdate `json:"update"`
}

// promptText concatenates every text content block's text (in order),
// ignoring block types NuimanBot doesn't yet consume (image, resource
// links, etc. — see initialize's PromptCapabilities, which declares this
// honestly to the host).
func promptText(blocks []contentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (s *Server) handleSessionPrompt(ctx context.Context, msg rpcMessage) {
	var params sessionPromptParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.writeError(msg.ID, errCodeInvalidParams, "invalid session/prompt params: "+err.Error())
		return
	}

	sessVal, ok := s.sessions.Load(params.SessionID)
	if !ok {
		s.writeError(msg.ID, errCodeInvalidParams, "unknown sessionId: "+params.SessionID)
		return
	}
	sess := sessVal.(*session)

	text := promptText(params.Prompt)
	if text == "" {
		s.writeError(msg.ID, errCodeInvalidParams, "session/prompt: no text content in prompt")
		return
	}

	promptCtx, cancel := context.WithCancel(ctx)
	sess.mu.Lock()
	sess.cancel = cancel
	sess.mu.Unlock()
	defer func() {
		sess.mu.Lock()
		sess.cancel = nil
		sess.mu.Unlock()
		cancel()
	}()

	incoming := &domain.IncomingMessage{
		ID:          fmt.Sprintf("acp-%s-%d", params.SessionID, atomic.AddUint64(&s.reqCounter, 1)),
		Platform:    domain.PlatformACP,
		PlatformUID: sess.platformUID,
		Text:        text,
		Timestamp:   time.Now(),
	}

	reply, err := s.chat.ProcessMessage(promptCtx, incoming)
	if err != nil {
		if errors.Is(promptCtx.Err(), context.Canceled) {
			s.writeResult(msg.ID, sessionPromptResult{StopReason: "cancelled"})
			return
		}
		s.writeError(msg.ID, errCodeInternal, "chat processing failed: "+err.Error())
		return
	}

	s.writeNotification(methodSessionUpdate, sessionUpdateParams{
		SessionID: params.SessionID,
		Update: sessionUpdate{
			SessionUpdate: agentMessageChunkTag,
			Content:       contentBlock{Type: "text", Text: reply.Content},
		},
	})

	s.writeResult(msg.ID, sessionPromptResult{StopReason: "end_turn"})
}

type sessionCancelParams struct {
	SessionID string `json:"sessionId"`
}

func (s *Server) handleSessionCancel(msg rpcMessage) {
	var params sessionCancelParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.logger.Warn("acp: invalid session/cancel params, ignoring", "error", err)
		return
	}
	sessVal, ok := s.sessions.Load(params.SessionID)
	if !ok {
		return
	}
	sess := sessVal.(*session)
	sess.mu.Lock()
	cancel := sess.cancel
	sess.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	s.write(rpcResponse{JSONRPC: jsonrpcVersion, ID: id, Result: result})
}

func (s *Server) writeError(id json.RawMessage, code int, message string) {
	s.write(rpcResponse{JSONRPC: jsonrpcVersion, ID: id, Error: &rpcError{Code: code, Message: message}})
}

func (s *Server) writeNotification(method string, params any) {
	s.write(rpcNotification{JSONRPC: jsonrpcVersion, Method: method, Params: params})
}

// write marshals v to a single JSON-RPC frame and issues exactly one Write
// call for it (frame bytes + trailing newline) — callers depend on this
// one-Write-per-frame guarantee (see acp_test.go's lineWriter).
func (s *Server) write(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		s.logger.Error("acp: failed to marshal outgoing message", "error", err)
		return
	}
	b = append(b, '\n')

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.out.Write(b); err != nil {
		s.logger.Error("acp: failed to write outgoing message", "error", err)
	}
}
