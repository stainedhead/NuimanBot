package persona

import (
	"context"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

// BenchmarkPromptComposer_SmallFiles benchmarks composition with small persona files (<500 chars each).
func BenchmarkPromptComposer_SmallFiles(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, "Be safe and secure.")
	repo.addFile("bench-user", domain.PersonaFileSOUL, "You are helpful and friendly.")
	repo.addFile("bench-user", domain.PersonaFileUSER, "Name: Test User\nTimezone: UTC")

	composer := NewPromptComposer(repo, "Global policy.")
	input := ComposerInput{UserID: "bench-user", Platform: "test"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := composer.Compose(ctx, input)
		if err != nil {
			b.Fatalf("Compose failed: %v", err)
		}
	}
}

// BenchmarkPromptComposer_MediumFiles benchmarks composition with medium files (~2000 chars each = ~500 tokens).
func BenchmarkPromptComposer_MediumFiles(b *testing.B) {
	repo := newMockRepo()

	mediumContent := strings.Repeat("This is a medium-length personality description. ", 40) // ~2000 chars

	repo.addFile("bench-user", domain.PersonaFileRULES, mediumContent)
	repo.addFile("bench-user", domain.PersonaFileSOUL, mediumContent)
	repo.addFile("bench-user", domain.PersonaFileUSER, mediumContent)

	composer := NewPromptComposer(repo, "Global policy.")
	input := ComposerInput{UserID: "bench-user", Platform: "test"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := composer.Compose(ctx, input)
		if err != nil {
			b.Fatalf("Compose failed: %v", err)
		}
	}
}

// BenchmarkPromptComposer_LargeFilesWithTruncation benchmarks composition with large files requiring truncation.
func BenchmarkPromptComposer_LargeFilesWithTruncation(b *testing.B) {
	repo := newMockRepo()

	largeContent := strings.Repeat("This is a very long personality description with lots of details. ", 100) // ~6600 chars = ~1650 tokens

	repo.addFile("bench-user", domain.PersonaFileRULES, largeContent)
	repo.addFile("bench-user", domain.PersonaFileSOUL, largeContent)
	repo.addFile("bench-user", domain.PersonaFileUSER, largeContent)

	// Strict budget to force truncation
	composer := NewPromptComposer(
		repo,
		"Global policy.",
		WithTokenBudget(TokenBudget{MaxTotal: 1000, MaxPerFile: 500}),
	)
	input := ComposerInput{UserID: "bench-user", Platform: "test"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := composer.Compose(ctx, input)
		if err != nil {
			b.Fatalf("Compose failed: %v", err)
		}
	}
}

// BenchmarkPromptComposer_NoFiles benchmarks composition when no persona files exist (graceful degradation).
func BenchmarkPromptComposer_NoFiles(b *testing.B) {
	repo := newMockRepo() // Empty repo

	composer := NewPromptComposer(repo, "Global policy.")
	input := ComposerInput{UserID: "nonexistent-user", Platform: "test"}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := composer.Compose(ctx, input)
		if err != nil {
			b.Fatalf("Compose failed: %v", err)
		}
	}
}

// BenchmarkPromptComposer_Parallel benchmarks concurrent composition requests.
func BenchmarkPromptComposer_Parallel(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, "Be safe.")
	repo.addFile("bench-user", domain.PersonaFileSOUL, "You are helpful.")
	repo.addFile("bench-user", domain.PersonaFileUSER, "Name: Test")

	composer := NewPromptComposer(repo, "Global policy.")
	input := ComposerInput{UserID: "bench-user", Platform: "test"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, err := composer.Compose(ctx, input)
			if err != nil {
				b.Fatalf("Compose failed: %v", err)
			}
		}
	})
}
