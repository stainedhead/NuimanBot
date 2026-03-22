package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// StdioTransport implements the Transport interface by communicating with a subprocess
// over stdin/stdout using newline-delimited JSON-RPC 2.0 messages.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	mu     sync.Mutex
}

// NewStdioTransport starts the given command as a subprocess and returns a StdioTransport
// that communicates with it over stdin/stdout.
// Returns an error if the command cannot be started.
func NewStdioTransport(command string, args []string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdio transport: open stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdio transport: open stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: stdio transport: start process: %w", err)
	}

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
	}, nil
}

// Send writes a newline-terminated JSON-RPC 2.0 message to the subprocess stdin
// and reads a response from stdout. The context is used to cancel in-flight operations.
func (t *StdioTransport) Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Build request with a fixed ID (stdio is synchronous, one request at a time)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: stdio transport: marshal request: %w", err)
	}
	// Append newline delimiter
	reqBytes = append(reqBytes, '\n')

	// Write to stdin in a goroutine so we can respect context cancellation
	writeErrCh := make(chan error, 1)
	go func() {
		_, err := t.stdin.Write(reqBytes)
		writeErrCh <- err
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("mcp: stdio transport: write cancelled: %w", ctx.Err())
	case err := <-writeErrCh:
		if err != nil {
			return nil, fmt.Errorf("mcp: stdio transport: write to stdin: %w", err)
		}
	}

	// Read response line in a goroutine so we can respect context cancellation
	type readResult struct {
		line []byte
		err  error
	}
	readCh := make(chan readResult, 1)
	go func() {
		line, err := t.reader.ReadBytes('\n')
		readCh <- readResult{line: line, err: err}
	}()

	var line []byte
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("mcp: stdio transport: read cancelled: %w", ctx.Err())
	case res := <-readCh:
		if res.err != nil {
			return nil, fmt.Errorf("mcp: stdio transport: read from stdout: %w", res.err)
		}
		line = res.line
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(line, &rpcResp); err != nil {
		return nil, fmt.Errorf("mcp: stdio transport: decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp: stdio transport: rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// Close terminates the subprocess and releases associated resources.
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Close stdin to signal EOF to the subprocess
	_ = t.stdin.Close()

	// Kill the process if it hasn't exited
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	// Wait to reap the process (avoid zombie)
	_ = t.cmd.Wait()
	return nil
}
