package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPTransport_SendReceive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}

		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}
		result := map[string]string{"status": "ok"}
		raw, _ := json.Marshal(result)
		resp.Result = json.RawMessage(raw)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer srv.Close()

	transport := NewHTTPTransport(srv.URL, nil)
	params, _ := json.Marshal(map[string]string{"key": "val"})

	result, err := transport.Send(context.Background(), "test/method", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf("expected status ok, got %q", got["status"])
	}
}

func TestHTTPTransport_HeadersIncluded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Custom") != "custom-value" {
			t.Errorf("expected X-Custom header, got %q", r.Header.Get("X-Custom"))
		}

		w.Header().Set("Content-Type", "application/json")
		resp := jsonRPCResponse{JSONRPC: "2.0", ID: 1}
		resp.Result = json.RawMessage(`{"ok":true}`)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer srv.Close()

	headers := map[string]string{
		"Authorization": "Bearer test-token",
		"X-Custom":      "custom-value",
	}
	transport := NewHTTPTransport(srv.URL, headers)
	params, _ := json.Marshal(map[string]any{})

	_, err := transport.Send(context.Background(), "test", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPTransport_Non200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	transport := NewHTTPTransport(srv.URL, nil)
	params, _ := json.Marshal(map[string]any{})

	_, err := transport.Send(context.Background(), "test", params)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestHTTPTransport_Non404Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer srv.Close()

	transport := NewHTTPTransport(srv.URL, nil)
	params, _ := json.Marshal(map[string]any{})

	_, err := transport.Send(context.Background(), "test", params)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestHTTPTransport_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json {{{`))
	}))
	defer srv.Close()

	transport := NewHTTPTransport(srv.URL, nil)
	params, _ := json.Marshal(map[string]any{})

	_, err := transport.Send(context.Background(), "test", params)
	if err == nil {
		t.Fatal("expected error for malformed JSON response, got nil")
	}
}

func TestHTTPTransport_JSONRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer srv.Close()

	transport := NewHTTPTransport(srv.URL, nil)
	params, _ := json.Marshal(map[string]any{})

	_, err := transport.Send(context.Background(), "unknown/method", params)
	if err == nil {
		t.Fatal("expected error for JSON-RPC error response, got nil")
	}
}

func TestHTTPTransport_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context allows
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer srv.Close()

	transport := NewHTTPTransport(srv.URL, nil)
	params, _ := json.Marshal(map[string]any{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := transport.Send(ctx, "test", params)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestHTTPTransport_Close(t *testing.T) {
	transport := NewHTTPTransport("http://example.com", nil)
	if err := transport.Close(); err != nil {
		t.Errorf("unexpected error on Close: %v", err)
	}
}

func TestHTTPTransport_DefaultTimeout(t *testing.T) {
	transport := NewHTTPTransport("http://example.com", nil)
	if transport.timeout != 30*time.Second {
		t.Errorf("expected default timeout of 30s, got %v", transport.timeout)
	}
}
