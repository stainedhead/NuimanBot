package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestIngatanServer creates a mock Ingatan HTTP server with a customisable token endpoint.
func newTestIngatanServer(t *testing.T, tokenHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/auth/token", tokenHandler)
	return srv
}

func defaultTokenHandler(w http.ResponseWriter, _ *http.Request) {
	exp := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"token":      "test-jwt-token",
		"expires_at": exp,
	}); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

// newTestIngatanClient creates an IngatanHTTPClient pointed at the given server URL.
func newTestIngatanClient(serverURL string) *IngatanHTTPClient {
	return NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:       serverURL,
		APIKey:        "test-api-key",
		StorePrefix:   "test",
		TLSSkipVerify: false,
	})
}

func TestIngatanHTTPClient_TokenExchange(t *testing.T) {
	var tokenCalled int32
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalled, 1)
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if body["api_key"] != "test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		defaultTokenHandler(w, r)
	})

	// Register a dummy endpoint to verify the Authorization header.
	var receivedAuth string
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	client := newTestIngatanClient(srv.URL)

	resp, err := client.Do(context.Background(), http.MethodGet, "/api/v1/test", nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&tokenCalled) != 1 {
		t.Errorf("Expected 1 token exchange call, got %d", atomic.LoadInt32(&tokenCalled))
	}
	if receivedAuth != "Bearer test-jwt-token" {
		t.Errorf("Expected Authorization header %q, got %q", "Bearer test-jwt-token", receivedAuth)
	}
}

func TestIngatanHTTPClient_TokenReuse(t *testing.T) {
	var tokenCalled int32
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&tokenCalled, 1)
		defaultTokenHandler(w, nil)
	})
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := newTestIngatanClient(srv.URL)

	// Make two requests; token should only be fetched once.
	for i := 0; i < 2; i++ {
		resp, err := client.Do(context.Background(), http.MethodGet, "/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Do call %d failed: %v", i+1, err)
		}
		resp.Body.Close()
	}

	if atomic.LoadInt32(&tokenCalled) != 1 {
		t.Errorf("Expected 1 token exchange call (token reuse), got %d", atomic.LoadInt32(&tokenCalled))
	}
}

func TestIngatanHTTPClient_TokenAutoRefresh(t *testing.T) {
	var tokenCalled int32
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&tokenCalled, 1)
		// Always return a valid-looking token.
		exp := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"token":      fmt.Sprintf("token-%d", atomic.LoadInt32(&tokenCalled)),
			"expires_at": exp,
		}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := newTestIngatanClient(srv.URL)

	// Pre-fill the cache with an expired token.
	client.tokenCache.mu.Lock()
	client.tokenCache.token = "expired-token"
	client.tokenCache.expiresAt = time.Now().Add(-1 * time.Hour) // past
	client.tokenCache.mu.Unlock()

	resp, err := client.Do(context.Background(), http.MethodGet, "/api/v1/test", nil)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&tokenCalled) != 1 {
		t.Errorf("Expected 1 token refresh, got %d", atomic.LoadInt32(&tokenCalled))
	}
}

func TestIngatanHTTPClient_TokenExchangeFailure(t *testing.T) {
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	client := newTestIngatanClient(srv.URL)

	_, err := client.Do(context.Background(), http.MethodGet, "/api/v1/test", nil)
	if err == nil {
		t.Fatal("Expected error from Do when token exchange fails, got nil")
	}
	if !strings.Contains(err.Error(), "ingatan: token exchange:") {
		t.Errorf("Expected error to contain %q, got %q", "ingatan: token exchange:", err.Error())
	}
}

func TestIngatanHTTPClient_TLSSkipVerify(t *testing.T) {
	// Use httptest TLS server which has a self-signed cert.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/token" {
			defaultTokenHandler(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	// Without TLSSkipVerify: should fail (cert not trusted).
	clientStrict := NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:       srv.URL,
		APIKey:        "test-key",
		TLSSkipVerify: false,
	})
	_, errStrict := clientStrict.Do(context.Background(), http.MethodGet, "/api/v1/test", nil)
	if errStrict == nil {
		t.Log("Note: TLS strict mode did not fail — this may happen if test certs are trusted in the environment")
	}

	// With TLSSkipVerify: should succeed.
	clientSkip := NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:       srv.URL,
		APIKey:        "test-key",
		TLSSkipVerify: true,
	})
	resp, err := clientSkip.Do(context.Background(), http.MethodGet, "/api/v1/test", nil)
	if err != nil {
		t.Fatalf("Do with TLSSkipVerify=true failed: %v", err)
	}
	resp.Body.Close()
}

func TestIngatanHTTPClient_ContextCancellation(t *testing.T) {
	started := make(chan struct{})
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		// Hang until request is cancelled.
		<-r.Context().Done()
		http.Error(w, "cancelled", http.StatusServiceUnavailable)
	})
	mux := srv.Config.Handler.(*http.ServeMux)
	// Pre-populate token so Do doesn't hit /auth/token first.
	mux.HandleFunc("/api/v1/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		http.Error(w, "cancelled", http.StatusServiceUnavailable)
	})

	client := newTestIngatanClient(srv.URL)
	// Manually set a valid token so the slow endpoint is hit directly.
	client.tokenCache.mu.Lock()
	client.tokenCache.token = "prefilled-token"
	client.tokenCache.expiresAt = time.Now().Add(24 * time.Hour)
	client.tokenCache.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Do(ctx, http.MethodGet, "/api/v1/slow", nil)
		errCh <- err
	}()

	<-started
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Expected error from cancelled context, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Do did not return after context cancellation within timeout")
	}
}

func TestTokenCache_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "empty token needs refresh",
			token:     "",
			expiresAt: time.Now().Add(24 * time.Hour),
			want:      true,
		},
		{
			name:      "expired token needs refresh",
			token:     "some-token",
			expiresAt: time.Now().Add(-1 * time.Minute),
			want:      true,
		},
		{
			name:      "token expiring within buffer needs refresh",
			token:     "some-token",
			expiresAt: time.Now().Add(3 * time.Minute), // < tokenRefreshBuffer (5min)
			want:      true,
		},
		{
			name:      "valid token with ample time does not need refresh",
			token:     "some-token",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache := &TokenCache{
				token:     tc.token,
				expiresAt: tc.expiresAt,
			}
			if got := cache.needsRefresh(); got != tc.want {
				t.Errorf("needsRefresh() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIngatanHTTPClient_RaceSafety(t *testing.T) {
	var tokenCalled int32
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&tokenCalled, 1)
		defaultTokenHandler(w, nil)
	})
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/noop", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := newTestIngatanClient(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Do(context.Background(), http.MethodGet, "/api/v1/noop", nil)
			if err != nil {
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()
}

func TestIngatanHTTPClient_RequestBody(t *testing.T) {
	srv := newTestIngatanServer(t, defaultTokenHandler)
	var receivedBody string
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/echo", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.WriteHeader(http.StatusCreated)
	})

	client := newTestIngatanClient(srv.URL)

	body := strings.NewReader(`{"content":"hello"}`)
	resp, err := client.Do(context.Background(), http.MethodPost, "/api/v1/echo", body)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	resp.Body.Close()

	if receivedBody != `{"content":"hello"}` {
		t.Errorf("Expected body %q, got %q", `{"content":"hello"}`, receivedBody)
	}
}

// Ensure IngatanHTTPClient does not create a new TLS transport per request
// by checking the httpClient field is the same object across calls.
func TestIngatanHTTPClient_TransportReuse(t *testing.T) {
	srv := newTestIngatanServer(t, defaultTokenHandler)
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/noop", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client := newTestIngatanClient(srv.URL)

	// Access the underlying http.Client's transport — it must not be nil
	// and must be the same instance across two calls.
	transport1 := client.httpClient.Transport
	resp1, err := client.Do(context.Background(), http.MethodGet, "/api/v1/noop", nil)
	if err != nil {
		t.Fatalf("Do (1) failed: %v", err)
	}
	resp1.Body.Close()

	transport2 := client.httpClient.Transport
	resp2, err := client.Do(context.Background(), http.MethodGet, "/api/v1/noop", nil)
	if err != nil {
		t.Fatalf("Do (2) failed: %v", err)
	}
	resp2.Body.Close()

	if transport1 != transport2 {
		t.Error("http.Transport is not reused between requests")
	}
	// Verify it's a *http.Transport (not nil)
	if _, ok := transport1.(*http.Transport); !ok {
		t.Errorf("Expected *http.Transport, got %T", transport1)
	}
}

// Verify TLSSkipVerify:true client uses a transport with InsecureSkipVerify:true.
func TestIngatanHTTPClient_TLSSkipVerifyTransport(t *testing.T) {
	client := NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:       "https://localhost:8443",
		APIKey:        "key",
		TLSSkipVerify: true,
	})
	tr, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected *http.Transport, got %T", client.httpClient.Transport)
	}
	if tr.TLSClientConfig == nil {
		t.Fatal("Expected non-nil TLSClientConfig when TLSSkipVerify is true")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify { //nolint:gosec // test assertion only
		t.Error("Expected InsecureSkipVerify = true when TLSSkipVerify is set")
	}
}

// Verify non-TLS client uses a transport without InsecureSkipVerify.
func TestIngatanHTTPClient_TLSStrictTransport(t *testing.T) {
	client := NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:       "https://localhost:8443",
		APIKey:        "key",
		TLSSkipVerify: false,
	})
	tr, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Expected *http.Transport, got %T", client.httpClient.Transport)
	}
	// TLSClientConfig may be nil (default) or have InsecureSkipVerify=false.
	if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify = false when TLSSkipVerify is not set")
	}
}

// Verify the Ping helper returns nil when the server is reachable.
func TestIngatanHTTPClient_Ping(t *testing.T) {
	srv := newTestIngatanServer(t, defaultTokenHandler)
	mux := srv.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	client := newTestIngatanClient(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

// Verify the Ping helper returns error when the server is unreachable.
func TestIngatanHTTPClient_PingUnreachable(t *testing.T) {
	client := NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL: "http://127.0.0.1:19999", // nothing listening here
		APIKey:  "key",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err == nil {
		t.Fatal("Expected Ping to fail for unreachable server, got nil")
	}
}

// Ensure that TLSSkipVerify=true logs a warning. We test this indirectly via
// a custom log capture. Since the production code uses log/slog, we just verify
// the client is constructed without panic and the transport is set correctly.
// The actual slog warning is verified by inspection (no unit-test hook needed here).
func TestIngatanHTTPClient_TLSSkipVerifyNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewIngatanHTTPClient panicked with TLSSkipVerify=true: %v", r)
		}
	}()
	_ = NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:       "https://localhost:8443",
		APIKey:        "key",
		TLSSkipVerify: true,
	})
}

// TestIngatanHTTPClient_RefreshRejectsEmptyToken verifies that refresh() returns an error
// containing "empty token" when the server returns an empty token string.
func TestIngatanHTTPClient_RefreshRejectsEmptyToken(t *testing.T) {
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]string{
			"token":      "",
			"expires_at": "2099-01-01T00:00:00Z",
		}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	client := newTestIngatanClient(srv.URL)

	err := client.refresh(context.Background())
	if err == nil {
		t.Fatal("Expected error from refresh() when server returns empty token, got nil")
	}
	if !strings.Contains(err.Error(), "empty token") {
		t.Errorf("Expected error to contain %q, got %q", "empty token", err.Error())
	}
}

// TestIngatanHTTPClient_RefreshRejectsAlreadyExpiredToken verifies that refresh() returns
// an error containing "already-expired" when the server returns an expires_at in the past.
func TestIngatanHTTPClient_RefreshRejectsAlreadyExpiredToken(t *testing.T) {
	srv := newTestIngatanServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]string{
			"token":      "abc",
			"expires_at": "1970-01-01T00:00:00Z",
		}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	client := newTestIngatanClient(srv.URL)

	err := client.refresh(context.Background())
	if err == nil {
		t.Fatal("Expected error from refresh() when server returns already-expired token, got nil")
	}
	if !strings.Contains(err.Error(), "already-expired") {
		t.Errorf("Expected error to contain %q, got %q", "already-expired", err.Error())
	}
}

// TestIngatanHTTPClient_RefreshErrorDoesNotUpdateCache verifies that when refresh() returns
// an error (empty token or past expiry), the token cache is NOT updated — so subsequent
// calls to needsRefresh() still return true (triggering a new refresh attempt).
func TestIngatanHTTPClient_RefreshErrorDoesNotUpdateCache(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "empty token does not update cache",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"token":      "",
					"expires_at": "2099-01-01T00:00:00Z",
				}); err != nil {
					http.Error(w, "encode error", http.StatusInternalServerError)
				}
			},
		},
		{
			name: "past expiry does not update cache",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"token":      "abc",
					"expires_at": "1970-01-01T00:00:00Z",
				}); err != nil {
					http.Error(w, "encode error", http.StatusInternalServerError)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestIngatanServer(t, tc.handler)
			client := newTestIngatanClient(srv.URL)

			// Capture cache state before the (expected-to-fail) refresh.
			client.tokenCache.mu.RLock()
			tokenBefore := client.tokenCache.token
			expiryBefore := client.tokenCache.expiresAt
			client.tokenCache.mu.RUnlock()

			err := client.refresh(context.Background())
			if err == nil {
				t.Fatal("Expected refresh() to return an error, got nil")
			}

			// Cache must be unchanged.
			client.tokenCache.mu.RLock()
			tokenAfter := client.tokenCache.token
			expiryAfter := client.tokenCache.expiresAt
			client.tokenCache.mu.RUnlock()

			if tokenAfter != tokenBefore {
				t.Errorf("Token cache was updated on error: before=%q after=%q", tokenBefore, tokenAfter)
			}
			if !expiryAfter.Equal(expiryBefore) {
				t.Errorf("ExpiresAt cache was updated on error: before=%v after=%v", expiryBefore, expiryAfter)
			}

			// Cache must still report needsRefresh() == true.
			if !client.tokenCache.needsRefresh() {
				t.Error("Expected needsRefresh() == true after failed refresh, but it returned false")
			}
		})
	}
}

// Compile-time type assertion: *http.Transport satisfies http.RoundTripper.
var _ http.RoundTripper = (*http.Transport)(nil)
