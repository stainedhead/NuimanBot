package domain

import (
	"strings"
	"testing"
	"time"
)

func TestDeriveConversationName_UsesFirstMessage(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 32, 0, 0, time.UTC)
	name := DeriveConversationName("Help me refactor the scheduler", now)
	if name != "Help me refactor the scheduler" {
		t.Fatalf("expected message text as name, got %q", name)
	}
}

func TestDeriveConversationName_EmptyFallsBackToTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 32, 0, 0, time.UTC)
	name := DeriveConversationName("", now)
	if name == "" {
		t.Fatal("fallback name must never be empty")
	}
	if !strings.Contains(name, "2026-08-05 14:32") {
		t.Fatalf("expected timestamp-based fallback name, got %q", name)
	}
}

func TestDeriveConversationName_WhitespaceOnlyFallsBack(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 32, 0, 0, time.UTC)
	name := DeriveConversationName("   \t\n  ", now)
	if name == "" {
		t.Fatal("fallback name must never be empty")
	}
	if strings.TrimSpace(name) == "" {
		t.Fatal("fallback name must not be whitespace-only")
	}
}

func TestDeriveConversationName_TruncatesLongMessages(t *testing.T) {
	now := time.Now()
	long := strings.Repeat("a", 200)
	name := DeriveConversationName(long, now)
	if len([]rune(name)) > 61 { // 60 chars + ellipsis rune
		t.Fatalf("expected truncated name, got length %d", len([]rune(name)))
	}
	if !strings.HasSuffix(name, "…") {
		t.Fatalf("expected truncated name to end with ellipsis, got %q", name)
	}
}

func TestFallbackConversationName_NeverEmpty(t *testing.T) {
	name := FallbackConversationName(time.Now())
	if name == "" {
		t.Fatal("FallbackConversationName must never return an empty string")
	}
}
