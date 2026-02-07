package cache_test

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/cache"
)

func TestLLMCache_SetAndGet(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()
	prompt := "What is 2+2?"
	response := &domain.LLMResponse{
		Content: "4",
		Usage: domain.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	// Set response
	c.Set(ctx, prompt, response)

	// Get response
	cached, found := c.Get(ctx, prompt)
	if !found {
		t.Fatal("Expected cache hit, got miss")
	}

	if cached.Content != response.Content {
		t.Errorf("Get() content = %q, want %q", cached.Content, response.Content)
	}

	if cached.Usage.TotalTokens != response.Usage.TotalTokens {
		t.Errorf("Get() tokens = %d, want %d", cached.Usage.TotalTokens, response.Usage.TotalTokens)
	}
}

func TestLLMCache_Miss(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()

	// Try to get non-existent entry
	_, found := c.Get(ctx, "non-existent prompt")
	if found {
		t.Error("Expected cache miss, got hit")
	}
}

func TestLLMCache_Expiration(t *testing.T) {
	c := cache.NewLLMCache(100, 100*time.Millisecond)

	ctx := context.Background()
	prompt := "What is the meaning of life?"
	response := &domain.LLMResponse{Content: "42"}

	// Set response
	c.Set(ctx, prompt, response)

	// Should be cached
	_, found := c.Get(ctx, prompt)
	if !found {
		t.Fatal("Expected cache hit immediately after set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = c.Get(ctx, prompt)
	if found {
		t.Error("Expected cache miss after expiration, got hit")
	}
}

func TestLLMCache_SizeLimit(t *testing.T) {
	// Create cache with size limit of 3
	c := cache.NewLLMCache(3, 1*time.Hour)

	ctx := context.Background()

	// Add 4 entries (exceeds limit)
	for i := 1; i <= 4; i++ {
		prompt := "prompt-" + string(rune('0'+i))
		response := &domain.LLMResponse{Content: "response"}
		c.Set(ctx, prompt, response)
	}

	// Get cache stats
	stats := c.Stats()

	// Should have at most 3 entries (oldest evicted)
	if stats.Size > 3 {
		t.Errorf("Cache size = %d, want <= 3", stats.Size)
	}
}

func TestLLMCache_CaseSensitive(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()
	response := &domain.LLMResponse{Content: "response"}

	c.Set(ctx, "Hello World", response)

	// Exact match should hit
	_, found := c.Get(ctx, "Hello World")
	if !found {
		t.Error("Expected cache hit for exact match")
	}

	// Different case should miss
	_, found = c.Get(ctx, "hello world")
	if found {
		t.Error("Expected cache miss for different case")
	}
}

func TestLLMCache_NormalizedKeys(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()
	response := &domain.LLMResponse{Content: "response"}

	// Set with extra whitespace
	c.Set(ctx, "  What is 2+2?  ", response)

	// Get with different whitespace (should normalize)
	_, found := c.Get(ctx, "What is 2+2?")
	if !found {
		t.Error("Expected cache hit after whitespace normalization")
	}
}

func TestLLMCache_Clear(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()

	// Add entries
	for i := 1; i <= 5; i++ {
		prompt := "prompt-" + string(rune('0'+i))
		response := &domain.LLMResponse{Content: "response"}
		c.Set(ctx, prompt, response)
	}

	stats := c.Stats()
	if stats.Size != 5 {
		t.Fatalf("Expected 5 entries before clear, got %d", stats.Size)
	}

	// Clear cache
	c.Clear()

	stats = c.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", stats.Size)
	}
}

func TestLLMCache_Stats(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()

	// Initial stats
	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Expected 0 hits/misses initially, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}

	// Add entry
	response := &domain.LLMResponse{Content: "response"}
	c.Set(ctx, "test", response)

	// Cache hit
	c.Get(ctx, "test")

	// Cache miss
	c.Get(ctx, "non-existent")

	// Check stats
	stats = c.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}

	// Check hit rate (1 hit out of 2 requests = 0.5)
	if stats.HitRate < 0.49 || stats.HitRate > 0.51 {
		t.Errorf("Expected hit rate ~0.5, got %f", stats.HitRate)
	}
}

func TestLLMCache_Concurrent(t *testing.T) {
	c := cache.NewLLMCache(1000, 1*time.Hour)

	ctx := context.Background()
	response := &domain.LLMResponse{Content: "response"}

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			prompt := "prompt-" + string(rune('0'+id))
			c.Set(ctx, prompt, response)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			prompt := "prompt-" + string(rune('0'+id))
			c.Get(ctx, prompt)
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have entries
	stats := c.Stats()
	if stats.Size == 0 {
		t.Error("Expected entries after concurrent operations")
	}
}

func TestLLMCache_Delete(t *testing.T) {
	c := cache.NewLLMCache(100, 1*time.Hour)

	ctx := context.Background()
	prompt := "test prompt"
	response := &domain.LLMResponse{Content: "response"}

	// Set entry
	c.Set(ctx, prompt, response)

	// Verify it exists
	_, found := c.Get(ctx, prompt)
	if !found {
		t.Fatal("Expected entry to exist")
	}

	// Delete entry
	c.Delete(ctx, prompt)

	// Verify it's gone
	_, found = c.Get(ctx, prompt)
	if found {
		t.Error("Expected entry to be deleted")
	}
}
