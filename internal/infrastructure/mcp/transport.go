package mcp

import (
	"context"
	"encoding/json"
)

// Transport defines the interface for sending and receiving MCP protocol messages.
// Implementations may use HTTP, stdio, or other transports.
type Transport interface {
	// Send sends a JSON-RPC request with the given method and params,
	// and returns the raw result field from the response.
	Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)

	// Close releases resources held by the transport, e.g. terminates a subprocess.
	Close() error
}
