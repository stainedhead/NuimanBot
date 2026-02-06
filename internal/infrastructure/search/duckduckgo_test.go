package search_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/infrastructure/search"
)

func TestNewClient(t *testing.T) {
	client := search.NewClient(10)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
}

func TestSearch_Success(t *testing.T) {
	// Mock DuckDuckGo HTML response
	mockHTML := `<html><body>
		<a class="result__a" href="https://example.com/1">Result 1</a>
		<a class="result__snippet">Snippet for result 1</a>
		<a class="result__a" href="https://example.com/2">Result 2</a>
		<a class="result__snippet">Snippet for result 2</a>
	</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q != "test query" {
			t.Errorf("Expected query 'test query', got '%s'", q)
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	client := search.NewClientWithBaseURL(10, server.URL)
	ctx := context.Background()

	results, err := client.Search(ctx, "test query", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected at least one result")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	client := search.NewClient(10)
	ctx := context.Background()

	_, err := client.Search(ctx, "", 5)
	if err == nil {
		t.Fatal("Expected error for empty query")
	}
}

func TestSearch_InvalidLimit(t *testing.T) {
	client := search.NewClient(10)
	ctx := context.Background()

	_, err := client.Search(ctx, "test", 0)
	if err == nil {
		t.Fatal("Expected error for invalid limit")
	}

	_, err = client.Search(ctx, "test", 100)
	if err == nil {
		t.Fatal("Expected error for limit > 50")
	}
}

func TestSearch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := search.NewClientWithBaseURL(10, server.URL)
	ctx := context.Background()

	_, err := client.Search(ctx, "test query", 5)
	if err == nil {
		t.Fatal("Expected error for HTTP 500")
	}
}
