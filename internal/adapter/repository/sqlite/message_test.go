package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"nuimanbot/internal/domain"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	schema := `
	CREATE TABLE conversations (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		platform TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT,
		tool_results TEXT,
		token_count INTEGER NOT NULL DEFAULT 0,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

// TestGetRecentMessages_EmptyConversation tests retrieving from non-existent conversation
func TestGetRecentMessages_EmptyConversation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	messages, err := repo.GetRecentMessages(ctx, "non-existent", 1000)
	if err != nil {
		t.Fatalf("Expected no error for empty conversation, got: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(messages))
	}
}

// TestGetRecentMessages_TokenLimit tests that messages are retrieved up to token limit
func TestGetRecentMessages_TokenLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-1"

	// Create conversation
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		convID, "user1", "cli", time.Now(), time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Insert messages with known token counts
	// Message 1: 100 tokens (oldest)
	// Message 2: 200 tokens
	// Message 3: 300 tokens
	// Message 4: 400 tokens (newest)
	baseTime := time.Now().Add(-1 * time.Hour)
	messages := []struct {
		id         string
		content    string
		tokens     int
		timeOffset time.Duration
	}{
		{"msg1", "First message", 100, 0},
		{"msg2", "Second message", 200, 10 * time.Minute},
		{"msg3", "Third message", 300, 20 * time.Minute},
		{"msg4", "Fourth message", 400, 30 * time.Minute},
	}

	for _, msg := range messages {
		_, err := db.Exec(
			`INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			msg.id, convID, "user", msg.content, "[]", "[]", msg.tokens, baseTime.Add(msg.timeOffset),
		)
		if err != nil {
			t.Fatalf("Failed to insert message %s: %v", msg.id, err)
		}
	}

	// Test 1: Retrieve with limit of 600 tokens (should get only last message: 400 <= 600)
	// Adding msg3 (300) would exceed: 400 + 300 = 700 > 600
	result, err := repo.GetRecentMessages(ctx, convID, 600)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 message for 600 token limit, got %d", len(result))
	}

	// Should return in chronological order (oldest first)
	if result[0].ID != "msg4" {
		t.Errorf("Expected message to be msg4, got %s", result[0].ID)
	}

	// Test 2: Retrieve with limit of 1000 tokens (should get all 4 messages)
	result, err = repo.GetRecentMessages(ctx, convID, 1000)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("Expected 4 messages for 1000 token limit, got %d", len(result))
	}

	// Verify chronological order
	if result[0].ID != "msg1" || result[1].ID != "msg2" || result[2].ID != "msg3" || result[3].ID != "msg4" {
		t.Errorf("Messages not in chronological order")
	}

	// Test 3: Retrieve with limit of 400 tokens (should get only last message)
	result, err = repo.GetRecentMessages(ctx, convID, 400)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 message for 400 token limit, got %d", len(result))
	}

	if result[0].ID != "msg4" {
		t.Errorf("Expected message to be msg4, got %s", result[0].ID)
	}

	// Test 4: Retrieve with limit of 50 tokens (should get 0 messages)
	result, err = repo.GetRecentMessages(ctx, convID, 50)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 0 {
		t.Fatalf("Expected 0 messages for 50 token limit, got %d", len(result))
	}
}

// TestGetRecentMessages_ChronologicalOrder verifies messages are returned oldest-first
func TestGetRecentMessages_ChronologicalOrder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-2"

	// Create conversation
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		convID, "user1", "cli", time.Now(), time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Insert messages in reverse order to test ordering
	baseTime := time.Now().Add(-1 * time.Hour)
	for i := 5; i >= 1; i-- {
		_, err := db.Exec(
			`INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"msg"+string(rune('0'+i)), convID, "user", "Message "+string(rune('0'+i)), "[]", "[]", 50, baseTime.Add(time.Duration(i)*time.Minute),
		)
		if err != nil {
			t.Fatalf("Failed to insert message: %v", err)
		}
	}

	result, err := repo.GetRecentMessages(ctx, convID, 1000)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 5 {
		t.Fatalf("Expected 5 messages, got %d", len(result))
	}

	// Verify chronological order (oldest to newest)
	for i := 0; i < 5; i++ {
		expectedID := "msg" + string(rune('1'+i))
		if result[i].ID != expectedID {
			t.Errorf("Message at position %d: expected %s, got %s", i, expectedID, result[i].ID)
		}
	}
}

// TestGetRecentMessages_ZeroTokenMessages tests handling of messages with zero tokens
func TestGetRecentMessages_ZeroTokenMessages(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-3"

	// Create conversation
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		convID, "user1", "cli", time.Now(), time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Insert messages with zero token counts
	baseTime := time.Now()
	for i := 1; i <= 3; i++ {
		_, err := db.Exec(
			`INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"msg"+string(rune('0'+i)), convID, "user", "Message", "[]", "[]", 0, baseTime.Add(time.Duration(i)*time.Minute),
		)
		if err != nil {
			t.Fatalf("Failed to insert message: %v", err)
		}
	}

	// With 0 token limit, should get no messages
	result, err := repo.GetRecentMessages(ctx, convID, 0)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 messages for 0 token limit, got %d", len(result))
	}

	// With any positive limit, should get all zero-token messages
	// (running total never exceeds limit since all messages are 0 tokens)
	result, err = repo.GetRecentMessages(ctx, convID, 1)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 messages for positive token limit with zero-token messages, got %d", len(result))
	}
}

// TestSaveMessage_Integration tests SaveMessage with the new signature
func TestSaveMessage_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-4"
	userID := "user123"
	platform := domain.PlatformCLI

	msg := domain.StoredMessage{
		ID:         "msg1",
		Role:       "user",
		Content:    "Test message",
		TokenCount: 10,
		Timestamp:  time.Now(),
	}

	// Save message (should create conversation automatically)
	err := repo.SaveMessage(ctx, convID, userID, platform, msg)
	if err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}

	// Verify conversation was created with correct user_id and platform
	var storedUserID, storedPlatform string
	err = db.QueryRow("SELECT user_id, platform FROM conversations WHERE id = ?", convID).Scan(&storedUserID, &storedPlatform)
	if err != nil {
		t.Fatalf("Failed to query conversation: %v", err)
	}

	if storedUserID != userID {
		t.Errorf("Expected user_id %s, got %s", userID, storedUserID)
	}

	if storedPlatform != string(platform) {
		t.Errorf("Expected platform %s, got %s", platform, storedPlatform)
	}

	// Verify message was saved
	messages, err := repo.GetRecentMessages(ctx, convID, 1000)
	if err != nil {
		t.Fatalf("GetRecentMessages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].ID != msg.ID {
		t.Errorf("Expected message ID %s, got %s", msg.ID, messages[0].ID)
	}
}
