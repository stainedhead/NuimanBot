package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockTransport is a test double for the Transport interface.
type mockTransport struct {
	sendFn func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
	closed bool
}

func (m *mockTransport) Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, method, params)
	}
	return nil, errors.New("mockTransport: no sendFn set")
}

func (m *mockTransport) Close() error {
	m.closed = true
	return nil
}

// newInitializeResponse builds a valid initialize response JSON.
func newInitializeResponse(protocolVersion string) json.RawMessage {
	resp := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]string{
			"name":    "test-server",
			"version": "1.0",
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestMCPClient_Initialize_Success(t *testing.T) {
	transport := &mockTransport{}
	transport.sendFn = func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if method != "initialize" {
			t.Errorf("expected initialize method, got %q", method)
		}

		// Verify required fields in the request
		var p map[string]any
		if err := json.Unmarshal(params, &p); err != nil {
			t.Errorf("failed to unmarshal params: %v", err)
		}
		if p["protocolVersion"] != "2024-11-05" {
			t.Errorf("expected protocolVersion 2024-11-05, got %v", p["protocolVersion"])
		}
		clientInfo, ok := p["clientInfo"].(map[string]any)
		if !ok {
			t.Error("expected clientInfo in params")
		} else if clientInfo["name"] != "nuimanbot" {
			t.Errorf("expected clientInfo.name 'nuimanbot', got %v", clientInfo["name"])
		}

		return newInitializeResponse("2024-11-05"), nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
}

func TestMCPClient_Initialize_CalledTwice(t *testing.T) {
	transport := &mockTransport{}
	transport.sendFn = func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		return newInitializeResponse("2024-11-05"), nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("first Initialize failed: %v", err)
	}

	// Second call should return an error
	err := client.Initialize(context.Background())
	if err == nil {
		t.Fatal("expected error on second Initialize, got nil")
	}
}

func TestMCPClient_Initialize_WrongProtocolVersion(t *testing.T) {
	transport := &mockTransport{}
	transport.sendFn = func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		return newInitializeResponse("2023-01-01"), nil
	}

	client := NewMCPClient(transport, "test-server")
	err := client.Initialize(context.Background())
	if err == nil {
		t.Fatal("expected error for mismatched protocol version, got nil")
	}
}

func TestMCPClient_Initialize_StoresCapabilities(t *testing.T) {
	transport := &mockTransport{}
	transport.sendFn = func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		resp := map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": true},
			},
			"serverInfo": map[string]string{"name": "test"},
		}
		data, _ := json.Marshal(resp)
		return data, nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	client.mu.RLock()
	caps := client.capabilities
	client.mu.RUnlock()

	if caps.Tools == nil {
		t.Error("expected Tools capability to be set")
	} else if !caps.Tools.ListChanged {
		t.Error("expected Tools.ListChanged to be true")
	}
}

func TestMCPClient_ListTools_NotInitialized(t *testing.T) {
	transport := &mockTransport{}
	client := NewMCPClient(transport, "test-server")

	_, err := client.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error when not initialized, got nil")
	}
}

func TestMCPClient_ListTools_Success(t *testing.T) {
	transport := &mockTransport{}
	callCount := 0
	transport.sendFn = func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			// initialize
			return newInitializeResponse("2024-11-05"), nil
		}
		// tools/list
		if method != "tools/list" {
			return nil, errors.New("unexpected method: " + method)
		}
		tools := []MCPTool{
			{
				Name:        "memory_search",
				Description: "Search memories",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			{
				Name:        "memory_save",
				Description: "Save a memory",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		}
		result := map[string]any{"tools": tools}
		data, _ := json.Marshal(result)
		return data, nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "memory_search" {
		t.Errorf("expected first tool 'memory_search', got %q", tools[0].Name)
	}
	if tools[1].Name != "memory_save" {
		t.Errorf("expected second tool 'memory_save', got %q", tools[1].Name)
	}
}

func TestMCPClient_ListTools_EmptyList(t *testing.T) {
	transport := &mockTransport{}
	callCount := 0
	transport.sendFn = func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return newInitializeResponse("2024-11-05"), nil
		}
		result := map[string]any{"tools": []MCPTool{}}
		data, _ := json.Marshal(result)
		return data, nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected empty tools list, got %d tools", len(tools))
	}
}

func TestMCPClient_CallTool_NotInitialized(t *testing.T) {
	transport := &mockTransport{}
	client := NewMCPClient(transport, "test-server")

	_, err := client.CallTool(context.Background(), "memory_search", map[string]any{"query": "test"})
	if err == nil {
		t.Fatal("expected error when not initialized, got nil")
	}
}

func TestMCPClient_CallTool_Success(t *testing.T) {
	transport := &mockTransport{}
	callCount := 0
	transport.sendFn = func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return newInitializeResponse("2024-11-05"), nil
		}
		if method != "tools/call" {
			return nil, errors.New("unexpected method: " + method)
		}

		// Verify the request params
		var p map[string]any
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		if p["name"] != "memory_search" {
			return nil, errors.New("unexpected tool name")
		}
		args, ok := p["arguments"].(map[string]any)
		if !ok || args["query"] != "test query" {
			return nil, errors.New("unexpected arguments")
		}

		result := MCPToolResult{
			Content: []MCPContent{
				{Type: "text", Text: "search result"},
			},
			IsError: false,
		}
		data, _ := json.Marshal(result)
		return data, nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	toolResult, err := client.CallTool(context.Background(), "memory_search", map[string]any{"query": "test query"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if len(toolResult.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(toolResult.Content))
	}
	if toolResult.Content[0].Text != "search result" {
		t.Errorf("expected text 'search result', got %q", toolResult.Content[0].Text)
	}
}

func TestMCPClient_CallTool_IsErrorTrue(t *testing.T) {
	transport := &mockTransport{}
	callCount := 0
	transport.sendFn = func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return newInitializeResponse("2024-11-05"), nil
		}

		result := MCPToolResult{
			Content: []MCPContent{
				{Type: "text", Text: "tool error occurred"},
			},
			IsError: true,
		}
		data, _ := json.Marshal(result)
		return data, nil
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	_, err := client.CallTool(context.Background(), "failing_tool", map[string]any{})
	if err == nil {
		t.Fatal("expected error for isError: true, got nil")
	}
}

func TestMCPClient_CallTool_TransportError(t *testing.T) {
	transport := &mockTransport{}
	callCount := 0
	transport.sendFn = func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return newInitializeResponse("2024-11-05"), nil
		}
		return nil, errors.New("transport error")
	}

	client := NewMCPClient(transport, "test-server")
	if err := client.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	_, err := client.CallTool(context.Background(), "some_tool", map[string]any{})
	if err == nil {
		t.Fatal("expected error for transport failure, got nil")
	}
}

func TestMCPClient_RequestIDsIncrements(t *testing.T) {
	transport := &mockTransport{}
	var seenIDs []int64
	callCount := 0

	transport.sendFn = func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return newInitializeResponse("2024-11-05"), nil
		}
		// tools/list - just return empty
		result := map[string]any{"tools": []MCPTool{}}
		data, _ := json.Marshal(result)
		_ = seenIDs // used to avoid lint warning
		return data, nil
	}

	client := NewMCPClient(transport, "test-server")
	_ = client.Initialize(context.Background())

	// Make multiple calls and verify IDs differ
	id1 := client.nextID()
	id2 := client.nextID()
	id3 := client.nextID()

	if id1 == id2 || id2 == id3 {
		t.Error("request IDs should be unique and incrementing")
	}
	if id2 != id1+1 || id3 != id2+1 {
		t.Errorf("expected incrementing IDs: %d, %d, %d", id1, id2, id3)
	}
}
