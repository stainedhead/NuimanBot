package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

const mcpProtocolVersion = "2024-11-05"

// MCPTool describes a tool exposed by an MCP server.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPContent represents a single content item in a tool result.
type MCPContent struct {
	Type string `json:"type"` // "text" | "image" | "resource"
	Text string `json:"text"`
}

// MCPToolResult represents the response from a tools/call request.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// ServerCapabilities describes the capabilities reported by an MCP server during initialization.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability describes the tools capability of an MCP server.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPClient implements a JSON-RPC 2.0 client for the Model Context Protocol.
// It supports the initialize, tools/list, and tools/call methods.
type MCPClient struct {
	transport    Transport
	serverName   string
	capabilities ServerCapabilities
	mu           sync.RWMutex
	initialized  bool
	idCounter    atomic.Int64
}

// NewMCPClient creates a new MCPClient that communicates via the given Transport.
// Initialize must be called before ListTools or CallTool.
func NewMCPClient(transport Transport, serverName string) *MCPClient {
	return &MCPClient{
		transport:  transport,
		serverName: serverName,
	}
}

// nextID returns a unique, atomically incrementing request ID.
func (c *MCPClient) nextID() int64 {
	return c.idCounter.Add(1)
}

// Initialize performs the MCP initialization handshake with the server.
// It must be called exactly once before any other methods.
// Returns an error if called a second time or if the server returns an incompatible protocol version.
func (c *MCPClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return fmt.Errorf("mcp: client %q: already initialized", c.serverName)
	}

	initParams := map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"clientInfo": map[string]string{
			"name":    "nuimanbot",
			"version": "1.0",
		},
		"capabilities": map[string]any{},
	}

	paramsRaw, err := json.Marshal(initParams)
	if err != nil {
		return fmt.Errorf("mcp: initialize: marshal params: %w", err)
	}

	result, err := c.transport.Send(ctx, "initialize", paramsRaw)
	if err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}

	var initResult struct {
		ProtocolVersion string             `json:"protocolVersion"`
		Capabilities    ServerCapabilities `json:"capabilities"`
	}
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("mcp: initialize: decode result: %w", err)
	}

	if initResult.ProtocolVersion != mcpProtocolVersion {
		return fmt.Errorf("mcp: initialize: server %q returned protocol version %q, expected %q",
			c.serverName, initResult.ProtocolVersion, mcpProtocolVersion)
	}

	c.capabilities = initResult.Capabilities
	c.initialized = true
	return nil
}

// ListTools returns all tools advertised by the MCP server.
// Initialize must be called before ListTools.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	c.mu.RLock()
	initialized := c.initialized
	c.mu.RUnlock()

	if !initialized {
		return nil, fmt.Errorf("mcp: client %q: not initialized; call Initialize first", c.serverName)
	}

	paramsRaw, err := json.Marshal(map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("mcp: tools/list: marshal params: %w", err)
	}

	result, err := c.transport.Send(ctx, "tools/list", paramsRaw)
	if err != nil {
		return nil, fmt.Errorf("mcp: tools/list: %w", err)
	}

	var listResult struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, fmt.Errorf("mcp: tools/list: decode result: %w", err)
	}

	if listResult.Tools == nil {
		return []MCPTool{}, nil
	}
	return listResult.Tools, nil
}

// CallTool invokes a tool on the MCP server by name with the given arguments.
// Initialize must be called before CallTool.
// If the server returns isError: true in the result, CallTool returns a Go error.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (*MCPToolResult, error) {
	c.mu.RLock()
	initialized := c.initialized
	c.mu.RUnlock()

	if !initialized {
		return nil, fmt.Errorf("mcp: client %q: not initialized; call Initialize first", c.serverName)
	}

	callParams := map[string]any{
		"name":      name,
		"arguments": args,
	}
	paramsRaw, err := json.Marshal(callParams)
	if err != nil {
		return nil, fmt.Errorf("mcp: tools/call: marshal params: %w", err)
	}

	result, err := c.transport.Send(ctx, "tools/call", paramsRaw)
	if err != nil {
		return nil, fmt.Errorf("mcp: tools/call %q: %w", name, err)
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, fmt.Errorf("mcp: tools/call %q: decode result: %w", name, err)
	}

	if toolResult.IsError {
		msg := "tool returned error"
		if len(toolResult.Content) > 0 {
			msg = toolResult.Content[0].Text
		}
		return nil, fmt.Errorf("mcp: tools/call %q on server %q: %s", name, c.serverName, msg)
	}

	return &toolResult, nil
}
