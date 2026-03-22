package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

// HTTPTransport implements the Transport interface using HTTP POST requests.
// Each Send call encodes the request as JSON-RPC 2.0 and POSTs it to the configured URL.
type HTTPTransport struct {
	url        string
	headers    map[string]string
	httpClient *http.Client
	timeout    time.Duration
	idCounter  atomic.Int64
}

// NewHTTPTransport creates a new HTTPTransport that sends requests to the given URL.
// Extra headers (e.g. Authorization) are included in every request.
// The default timeout is 30 seconds.
func NewHTTPTransport(url string, headers map[string]string) *HTTPTransport {
	t := &HTTPTransport{
		url:     url,
		headers: headers,
		timeout: defaultHTTPTimeout,
	}
	t.httpClient = &http.Client{Timeout: t.timeout}
	return t
}

// Send encodes a JSON-RPC 2.0 request and sends it to the transport URL.
// It returns the raw result field from the response, or an error if the request fails
// or the server returns a non-2xx status or a JSON-RPC error.
func (t *HTTPTransport) Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := t.idCounter.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: http transport: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcp: http transport: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp: http transport: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp: http transport: server returned status %d", resp.StatusCode)
	}

	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("mcp: http transport: decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp: http transport: rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// Close is a no-op for the HTTP transport; it implements the Transport interface.
func (t *HTTPTransport) Close() error {
	return nil
}
