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

	// Enable foreign key constraints (required for CASCADE)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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

// TestInit tests the Init method (schema initialization)
func TestInit(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	// Init should not error even if tables already exist
	err := repo.Init(ctx)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify tables exist by querying them
	_, err = db.Exec("SELECT 1 FROM conversations LIMIT 1")
	if err != nil {
		t.Errorf("Conversations table should exist after Init: %v", err)
	}

	_, err = db.Exec("SELECT 1 FROM messages LIMIT 1")
	if err != nil {
		t.Errorf("Messages table should exist after Init: %v", err)
	}
}

// TestGetConversation tests retrieving a full conversation
func TestGetConversation(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-5"

	// Create conversation with messages
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		convID, "user1", "cli", time.Now(), time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Insert messages
	baseTime := time.Now()
	for i := 1; i <= 3; i++ {
		_, err := db.Exec(
			`INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"msg"+string(rune('0'+i)), convID, "user", "Message "+string(rune('0'+i)),
			"[]", "[]", 10, baseTime.Add(time.Duration(i)*time.Minute),
		)
		if err != nil {
			t.Fatalf("Failed to insert message: %v", err)
		}
	}

	// Test GetConversation
	conv, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if len(conv.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(conv.Messages))
	}

	// Verify chronological order
	for i := 0; i < 3; i++ {
		expectedID := "msg" + string(rune('1'+i))
		if conv.Messages[i].ID != expectedID {
			t.Errorf("Message at position %d: expected %s, got %s", i, expectedID, conv.Messages[i].ID)
		}
	}
}

// TestGetConversation_NotFound tests GetConversation with non-existent conversation
func TestGetConversation_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	conv, err := repo.GetConversation(ctx, "non-existent")
	if err == nil {
		t.Error("GetConversation should return error for non-existent conversation")
	}
	if conv != nil {
		t.Error("GetConversation should return nil conversation for non-existent ID")
	}
}

// TestGetConversation_WithToolCalls tests conversation retrieval with tool calls
func TestGetConversation_WithToolCalls(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-6"

	// Create conversation
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		convID, "user1", "cli", time.Now(), time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Insert message with tool calls and tool results
	toolCallsJSON := `[{"tool_name":"calculator","arguments":{"a":5,"b":3}}]`
	toolResultsJSON := `[{"tool_name":"calculator","output":"8"}]`

	_, err = db.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg1", convID, "assistant", "Using calculator", toolCallsJSON, toolResultsJSON, 20, time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to insert message: %v", err)
	}

	conv, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if len(conv.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conv.Messages))
	}

	// Verify tool calls and results are populated
	// The actual parsing is done in GetConversation - we verify non-empty
	if len(conv.Messages[0].ToolCalls) == 0 {
		t.Error("Expected tool calls to be parsed")
	}
	if len(conv.Messages[0].ToolResults) == 0 {
		t.Error("Expected tool results to be parsed")
	}
}

// TestDeleteConversation tests conversation deletion
func TestDeleteConversation(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-7"

	// Create conversation with messages
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		convID, "user1", "cli", time.Now(), time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg1", convID, "user", "Test message", "[]", "[]", 10, time.Now(),
	)
	if err != nil {
		t.Fatalf("Failed to insert message: %v", err)
	}

	// Delete conversation
	err = repo.DeleteConversation(ctx, convID)
	if err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}

	// Verify conversation is deleted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM conversations WHERE id = ?", convID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query conversations: %v", err)
	}
	if count != 0 {
		t.Error("Expected conversation to be deleted")
	}

	// Verify messages are deleted (CASCADE)
	err = db.QueryRow("SELECT COUNT(*) FROM messages WHERE conversation_id = ?", convID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query messages: %v", err)
	}
	if count != 0 {
		t.Error("Expected messages to be deleted via CASCADE")
	}
}

// TestDeleteConversation_NotFound tests deleting non-existent conversation
func TestDeleteConversation_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	// Deleting non-existent conversation should not error
	err := repo.DeleteConversation(ctx, "non-existent")
	if err != nil {
		t.Errorf("DeleteConversation should not error for non-existent conversation: %v", err)
	}
}

// TestListConversations tests listing conversations for a user
func TestListConversations(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	userID := "user1"

	// Create multiple conversations for the user
	conversations := []struct {
		id       string
		platform string
	}{
		{"conv1", "cli"},
		{"conv2", "telegram"},
		{"conv3", "slack"},
	}

	baseTime := time.Now()
	for i, conv := range conversations {
		_, err := db.Exec(
			"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			conv.id, userID, conv.platform,
			baseTime.Add(time.Duration(i)*time.Minute),
			baseTime.Add(time.Duration(i)*time.Minute),
		)
		if err != nil {
			t.Fatalf("Failed to create conversation %s: %v", conv.id, err)
		}
	}

	// Create conversation for different user (should not be listed)
	_, err := db.Exec(
		"INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"other-conv", "user2", "cli", baseTime, baseTime,
	)
	if err != nil {
		t.Fatalf("Failed to create other user's conversation: %v", err)
	}

	// List conversations
	result, err := repo.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 conversations, got %d", len(result))
	}

	// Verify conversations are for the correct user
	for _, conv := range result {
		if conv.UserID != userID {
			t.Errorf("Expected user_id %s, got %s", userID, conv.UserID)
		}
	}

	// Verify ordering (most recent first - by updated_at DESC)
	// Last created should be first in results
	if result[0].ID != "conv3" {
		t.Errorf("Expected first conversation to be conv3 (most recent), got %s", result[0].ID)
	}
}

// TestListConversations_Empty tests listing for user with no conversations
func TestListConversations_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()

	result, err := repo.ListConversations(ctx, "non-existent-user")
	if err != nil {
		t.Fatalf("ListConversations should not error for user with no conversations: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 conversations, got %d", len(result))
	}
}

// TestSaveMessage_MultipleMessages tests saving multiple messages to same conversation
func TestSaveMessage_MultipleMessages(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-8"
	userID := "user1"
	platform := domain.PlatformCLI

	// Save first message (creates conversation)
	msg1 := domain.StoredMessage{
		ID:         "msg1",
		Role:       "user",
		Content:    "First message",
		TokenCount: 10,
		Timestamp:  time.Now(),
	}

	err := repo.SaveMessage(ctx, convID, userID, platform, msg1)
	if err != nil {
		t.Fatalf("SaveMessage failed for first message: %v", err)
	}

	// Save second message (conversation already exists)
	msg2 := domain.StoredMessage{
		ID:         "msg2",
		Role:       "assistant",
		Content:    "Second message",
		TokenCount: 15,
		Timestamp:  time.Now().Add(1 * time.Minute),
	}

	err = repo.SaveMessage(ctx, convID, userID, platform, msg2)
	if err != nil {
		t.Fatalf("SaveMessage failed for second message: %v", err)
	}

	// Verify both messages exist
	conv, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if len(conv.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(conv.Messages))
	}
}

// TestSaveMessage_WithToolCallsAndResults tests saving messages with tool data
func TestSaveMessage_WithToolCallsAndResults(t *testing.T) {
	db := setupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := NewMessageRepository(db)
	ctx := context.Background()
	convID := "test-conv-9"
	userID := "user1"
	platform := domain.PlatformCLI

	msg := domain.StoredMessage{
		ID:         "msg1",
		Role:       "assistant",
		Content:    "Using calculator",
		TokenCount: 20,
		Timestamp:  time.Now(),
		ToolCalls: []domain.ToolCall{
			{
				ToolName:  "calculator",
				Arguments: map[string]any{"a": 5, "b": 3},
			},
		},
		ToolResults: []domain.ToolResult{
			{
				ToolName: "calculator",
				Output:   "8",
			},
		},
	}

	err := repo.SaveMessage(ctx, convID, userID, platform, msg)
	if err != nil {
		t.Fatalf("SaveMessage with tool data failed: %v", err)
	}

	// Verify message was saved with tool data
	conv, err := repo.GetConversation(ctx, convID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if len(conv.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conv.Messages))
	}

	if len(conv.Messages[0].ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(conv.Messages[0].ToolCalls))
	}

	if len(conv.Messages[0].ToolResults) != 1 {
		t.Errorf("Expected 1 tool result, got %d", len(conv.Messages[0].ToolResults))
	}
}
