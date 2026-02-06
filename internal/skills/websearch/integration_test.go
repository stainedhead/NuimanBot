package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/domain"
	searchClient "nuimanbot/internal/infrastructure/search"
)

func TestExecute_WithMockServer_MultipleResults(t *testing.T) {
	// Create mock server that returns DuckDuckGo-style HTML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<body>
				<a class="result__a" href="https://example.com/1">First Result</a>
				<a class="result__snippet">First snippet text</a>
				<a class="result__a" href="https://example.com/2">Second Result</a>
				<a class="result__snippet">Second snippet text</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "test query",
		"limit": 5,
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output == "" {
		t.Error("Expected output to be non-empty")
	}

	if result.Metadata["query"] != "test query" {
		t.Errorf("Expected query 'test query', got %v", result.Metadata["query"])
	}

	if result.Metadata["count"] != 2 {
		t.Errorf("Expected count 2, got %v", result.Metadata["count"])
	}

	results, ok := result.Metadata["results"].([]map[string]any)
	if !ok {
		t.Fatal("Expected results in metadata")
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0]["title"] != "First Result" {
		t.Errorf("Expected first title 'First Result', got %v", results[0]["title"])
	}
}

func TestExecute_WithMockServer_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body>No results</body></html>`))
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "nonexistent query",
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Metadata["count"] != 0 {
		t.Errorf("Expected count 0, got %v", result.Metadata["count"])
	}
}

func TestExecute_WithMockServer_ResultsWithoutSnippets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<body>
				<a class="result__a" href="https://example.com/no-snippet">Result without snippet</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "test",
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	results, ok := result.Metadata["results"].([]map[string]any)
	if !ok {
		t.Fatal("Expected results in metadata")
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0]["snippet"] != "" {
		t.Errorf("Expected empty snippet, got %v", results[0]["snippet"])
	}
}

func TestExecute_WithMockServer_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "test",
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error for server failure")
	}
}

func TestExecute_WithMockServer_LimitAsFloat64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<body>
				<a class="result__a" href="https://example.com/1">Result 1</a>
				<a class="result__snippet">Snippet 1</a>
				<a class="result__a" href="https://example.com/2">Result 2</a>
				<a class="result__snippet">Snippet 2</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "test",
		"limit": float64(10), // JSON numbers decode as float64
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestExecute_WithMockServer_LimitAsInt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<body>
				<a class="result__a" href="https://example.com/1">Result 1</a>
				<a class="result__snippet">Snippet 1</a>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "test",
		"limit": 15, // int type
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestExecute_WithMockServer_ManyResults(t *testing.T) {
	// Create HTML with many results
	html := `<html><body>`
	for i := 1; i <= 20; i++ {
		html += `<a class="result__a" href="https://example.com/` + string(rune(i)) + `">Result ` + string(rune(i)) + `</a>`
		html += `<a class="result__snippet">Snippet ` + string(rune(i)) + `</a>`
	}
	html += `</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	client := searchClient.NewClientWithBaseURL(10, server.URL)
	w := &WebSearch{
		client: client,
		config: domain.SkillConfig{Enabled: true},
	}

	result, err := w.Execute(context.Background(), map[string]any{
		"query": "test",
		"limit": 20,
	})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	// Count should reflect actual parsed results
	count, ok := result.Metadata["count"].(int)
	if !ok {
		t.Fatal("Expected count in metadata")
	}

	if count == 0 {
		t.Error("Expected non-zero result count")
	}
}
