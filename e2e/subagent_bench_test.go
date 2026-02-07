package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/subagent"
)

// BenchmarkSubagent_ContextForking benchmarks context forking performance
func BenchmarkSubagent_ContextForking(b *testing.B) {
	// Create sample conversation history
	history := make([]domain.Message, 10)
	for i := 0; i < 10; i++ {
		history[i] = domain.Message{
			Role:    "user",
			Content: "Sample message for benchmarking",
		}
	}

	allowedTools := []string{"read", "write", "grep", "glob"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark forking logic (deep copy)
		copiedHistory := make([]domain.Message, len(history))
		for j, msg := range history {
			copiedHistory[j] = domain.Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		copiedTools := make([]string, len(allowedTools))
		copy(copiedTools, allowedTools)
	}
}

// BenchmarkSubagent_LifecycleOperations benchmarks lifecycle management
func BenchmarkSubagent_LifecycleOperations(b *testing.B) {
	mockExecutor := &mockSubagentExecutor{
		results: make(map[string]*domain.SubagentResult),
		delay:   1 * time.Millisecond, // Minimal delay
	}

	lifecycleManager := subagent.NewLifecycleManager(mockExecutor)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subagentID := fmt.Sprintf("bench-%d-%d", i, time.Now().UnixNano())

		subagentCtx := domain.SubagentContext{
			ID:              subagentID,
			ParentContextID: "parent",
			SkillName:       "bench-skill",
			AllowedTools:    []string{"read"},
			ResourceLimits:  domain.DefaultResourceLimits(),
			ConversationHistory: []domain.Message{
				{Role: "user", Content: "Benchmark"},
			},
			CreatedAt: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		// Start
		if err := lifecycleManager.Start(ctx, subagentCtx); err != nil {
			b.Fatalf("Start failed: %v", err)
		}

		// Wait for completion
		time.Sleep(5 * time.Millisecond)

		// GetStatus
		if _, err := lifecycleManager.GetStatus(ctx, subagentID); err != nil {
			b.Fatalf("GetStatus failed: %v", err)
		}
	}
}

// BenchmarkSubagent_ConcurrentExecution benchmarks parallel subagent execution
func BenchmarkSubagent_ConcurrentExecution(b *testing.B) {
	mockExecutor := &mockSubagentExecutor{
		results: make(map[string]*domain.SubagentResult),
		delay:   1 * time.Millisecond,
	}

	lifecycleManager := subagent.NewLifecycleManager(mockExecutor)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Start 10 concurrent subagents
		for j := 0; j < 10; j++ {
			subagentID := fmt.Sprintf("concurrent-%d-%d-%d", i, j, time.Now().UnixNano())

			subagentCtx := domain.SubagentContext{
				ID:              subagentID,
				ParentContextID: "parent",
				SkillName:       "concurrent-skill",
				AllowedTools:    []string{"read"},
				ResourceLimits:  domain.DefaultResourceLimits(),
				ConversationHistory: []domain.Message{
					{Role: "user", Content: "Concurrent test"},
				},
				CreatedAt: time.Now(),
				Metadata:  make(map[string]interface{}),
			}

			if err := lifecycleManager.Start(ctx, subagentCtx); err != nil {
				b.Fatalf("Start failed: %v", err)
			}
		}

		// Wait for all to complete
		time.Sleep(10 * time.Millisecond)
	}
}

// BenchmarkSubagent_ResourceLimitValidation benchmarks resource limit checks
func BenchmarkSubagent_ResourceLimitValidation(b *testing.B) {
	limits := domain.DefaultResourceLimits()
	tokensUsed := 50000
	toolCallsMade := 25

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate resource limit checks
		_ = tokensUsed > limits.MaxTokens
		_ = toolCallsMade >= limits.MaxToolCalls
	}
}
