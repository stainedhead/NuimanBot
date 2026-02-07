package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog" // Structured logging

	"time"

	"nuimanbot/internal/domain"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// MessageRepository implements domain.MemoryRepository for SQLite.
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a new SQLite message repository.
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Init initializes the conversations and messages tables if they don't exist.
func (r *MessageRepository) Init(ctx context.Context) error {
	const createConversationsTableSQL = `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		platform TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`
	_, err := r.db.ExecContext(ctx, createConversationsTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create conversations table: %w", err)
	}

	const createMessagesTableSQL = `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT, -- Stored as JSON
		tool_results TEXT, -- Stored as JSON
		token_count INTEGER NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`
	_, err = r.db.ExecContext(ctx, createMessagesTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}
	return nil
}

// SaveMessage persists a message in a conversation.
// If the conversation does not exist, it is created with the provided userID and platform.
func (r *MessageRepository) SaveMessage(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbe := tx.Rollback(); rbe != nil {
			slog.Error("Rollback error in SaveMessage", "error", rbe)
		}
	}()

	// Check if conversation exists, if not, create a new one.
	// This implicitly handles creating a conversation when the first message is saved.
	var count int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM conversations WHERE id = ?", convID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to query conversation existence: %w", err)
	}

	if count == 0 {
		// Create conversation with proper user and platform context
		_, err = tx.ExecContext(ctx,
			`INSERT INTO conversations (id, user_id, platform, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			convID, userID, platform, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("failed to insert new conversation: %w", err)
		}
	} else {
		// Update conversation's updated_at timestamp
		_, err = tx.ExecContext(ctx, "UPDATE conversations SET updated_at = ? WHERE id = ?", time.Now(), convID)
		if err != nil {
			return fmt.Errorf("failed to update conversation timestamp: %w", err)
		}
	}

	toolCallsJSON, err := json.Marshal(msg.ToolCalls)
	if err != nil {
		return fmt.Errorf("failed to marshal tool calls: %w", err)
	}
	toolResultsJSON, err := json.Marshal(msg.ToolResults)
	if err != nil {
		return fmt.Errorf("failed to marshal tool results: %w", err)
	}

	const insertMessageSQL = `
	INSERT INTO messages (id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?);`
	_, err = tx.ExecContext(
		ctx,
		insertMessageSQL,
		msg.ID,
		convID,
		msg.Role,
		msg.Content,
		toolCallsJSON,
		toolResultsJSON,
		msg.TokenCount,
		msg.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	return tx.Commit()
}

// GetConversation retrieves a full conversation.
func (r *MessageRepository) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	conversation := &domain.Conversation{ID: convID}
	var platformStr string

	err := r.db.QueryRowContext(ctx,
		"SELECT user_id, platform, created_at, updated_at FROM conversations WHERE id = ?", convID).
		Scan(&conversation.UserID, &platformStr, &conversation.CreatedAt, &conversation.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound // Using a generic "not found" error for conversations
		}
		return nil, fmt.Errorf("failed to get conversation %s: %w", convID, err)
	}
	conversation.Platform = domain.Platform(platformStr)

	rows, err := r.db.QueryContext(ctx,
		"SELECT id, role, content, tool_calls, tool_results, token_count, timestamp FROM messages WHERE conversation_id = ? ORDER BY timestamp ASC", convID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages for conversation %s: %w", convID, err)
	}
	defer func() {
		if cle := rows.Close(); cle != nil {
			slog.Error("Rows close error in GetConversation", "error", cle)
		}
	}()

	var messages []domain.StoredMessage
	for rows.Next() {
		msg := domain.StoredMessage{}
		var toolCallsJSON, toolResultsJSON []byte
		err := rows.Scan(
			&msg.ID,
			&msg.Role,
			&msg.Content,
			&toolCallsJSON,
			&toolResultsJSON,
			&msg.TokenCount,
			&msg.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if toolCallsJSON != nil {
			err = json.Unmarshal(toolCallsJSON, &msg.ToolCalls)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool calls for message %s: %w", msg.ID, err)
			}
		}
		if toolResultsJSON != nil {
			err = json.Unmarshal(toolResultsJSON, &msg.ToolResults)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool results for message %s: %w", msg.ID, err)
			}
		}
		messages = append(messages, msg)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}
	conversation.Messages = messages
	return conversation, nil
}

// GetRecentMessages retrieves messages up to a token limit.
// Uses a window function to calculate running token totals and stops fetching
// when the limit is reached. Returns messages in chronological order (oldest first).
func (r *MessageRepository) GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
	// Early return for zero or negative token limit
	if maxTokens <= 0 {
		return []domain.StoredMessage{}, nil
	}

	// Use a CTE with window function to calculate running token totals
	// in reverse chronological order, then filter and return in chronological order
	const selectSQL = `
	WITH cumulative_tokens AS (
		SELECT
			id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp,
			SUM(token_count) OVER (ORDER BY timestamp DESC) AS running_total
		FROM messages
		WHERE conversation_id = ?
		ORDER BY timestamp DESC
	)
	SELECT id, conversation_id, role, content, tool_calls, tool_results, token_count, timestamp
	FROM cumulative_tokens
	WHERE running_total <= ?
	ORDER BY timestamp ASC;`

	rows, err := r.db.QueryContext(ctx, selectSQL, convID, maxTokens)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent messages for conversation %s: %w", convID, err)
	}
	defer func() {
		if cle := rows.Close(); cle != nil {
			slog.Error("Rows close error in GetRecentMessages", "error", cle)
		}
	}()

	var messages []domain.StoredMessage
	for rows.Next() {
		var msg domain.StoredMessage
		var convID string
		var toolCallsJSON, toolResultsJSON []byte

		err := rows.Scan(
			&msg.ID,
			&convID,
			&msg.Role,
			&msg.Content,
			&toolCallsJSON,
			&toolResultsJSON,
			&msg.TokenCount,
			&msg.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}

		// Unmarshal tool calls and results
		if len(toolCallsJSON) > 0 && string(toolCallsJSON) != "null" {
			if err := json.Unmarshal(toolCallsJSON, &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool calls: %w", err)
			}
		}
		if len(toolResultsJSON) > 0 && string(toolResultsJSON) != "null" {
			if err := json.Unmarshal(toolResultsJSON, &msg.ToolResults); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool results: %w", err)
			}
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message rows: %w", err)
	}

	return messages, nil
}

// DeleteConversation removes a conversation and its associated messages.
func (r *MessageRepository) DeleteConversation(ctx context.Context, convID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbe := tx.Rollback(); rbe != nil {
			slog.Error("Rollback error in DeleteConversation", "error", rbe)
		}
	}()

	// Deleting from conversations table will cascade delete from messages table
	_, err = tx.ExecContext(ctx, "DELETE FROM conversations WHERE id = ?", convID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation %s: %w", convID, err)
	}

	return tx.Commit()
}

// ListConversations returns conversations for a user.
func (r *MessageRepository) ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error) {
	const selectSQL = `
	SELECT c.id, c.user_id, c.platform, c.created_at, c.updated_at,
	       (SELECT content FROM messages WHERE conversation_id = c.id ORDER BY timestamp DESC LIMIT 1) as last_message_snippet
	FROM conversations c
	WHERE c.user_id = ?
	ORDER BY c.updated_at DESC;`

	rows, err := r.db.QueryContext(ctx, selectSQL, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations for user %s: %w", userID, err)
	}
	defer func() {
		if cle := rows.Close(); cle != nil {
			slog.Error("Rows close error in ListConversations", "error", cle)
		}
	}()

	var summaries []domain.ConversationSummary
	for rows.Next() {
		summary := domain.ConversationSummary{}
		var platformStr sql.NullString // Use NullString to handle potential NULL from platform field if no messages yet
		var lastMessageSnippet sql.NullString

		err := rows.Scan(
			&summary.ID,
			&summary.UserID,
			&platformStr,
			&summary.CreatedAt,
			&summary.UpdatedAt,
			&lastMessageSnippet,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation summary: %w", err)
		}

		if platformStr.Valid {
			summary.Platform = domain.Platform(platformStr.String)
		} else {
			summary.Platform = "" // or a default unknown platform
		}
		if lastMessageSnippet.Valid {
			summary.LastMessageSnippet = lastMessageSnippet.String
		}
		summaries = append(summaries, summary)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversation summaries: %w", err)
	}
	return summaries, nil
}

// PoolStats returns database connection pool statistics.
// Useful for monitoring and performance tuning.
func (r *MessageRepository) PoolStats() sql.DBStats {
	return r.db.Stats()
}
