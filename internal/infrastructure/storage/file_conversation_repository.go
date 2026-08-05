package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"nuimanbot/internal/domain"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConversationIndex represents the index file for conversations
type ConversationIndex struct {
	Version       string                            `json:"version"`
	LastUpdated   string                            `json:"lastUpdated"`
	Conversations map[string]ConversationIndexEntry `json:"conversations"` // convID -> entry
}

// ConversationIndexEntry represents a single conversation in the index
type ConversationIndexEntry struct {
	ID                 string          `json:"id"`
	UserID             string          `json:"userID"`
	Platform           domain.Platform `json:"platform"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	MessageCount       int             `json:"messageCount"`
	LastMessageSnippet string          `json:"lastMessageSnippet"`
	FilePath           string          `json:"filePath"` // Relative path to messages.jsonl
}

// FileConversationRepository implements ConversationRepository using file storage
type FileConversationRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileConversationRepository creates a new file-based conversation repository
func NewFileConversationRepository(basePath string) *FileConversationRepository {
	return &FileConversationRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

// getUserConvDir returns the path to a user's conversations directory
func (r *FileConversationRepository) getUserConvDir(userID string) string {
	return filepath.Join(r.basePath, "users", userID, "conversations")
}

// getConvDir returns the path to a specific conversation directory
func (r *FileConversationRepository) getConvDir(userID, convID string) string {
	return filepath.Join(r.getUserConvDir(userID), convID)
}

// getMessagesFile returns the path to a conversation's messages.jsonl file
func (r *FileConversationRepository) getMessagesFile(userID, convID string) string {
	return filepath.Join(r.getConvDir(userID, convID), "messages.jsonl")
}

// getIndexFile returns the path to a user's conversation index file
func (r *FileConversationRepository) getIndexFile(userID string) string {
	return filepath.Join(r.getUserConvDir(userID), "index.json")
}

// loadIndex loads the conversation index for a user
func (r *FileConversationRepository) loadIndex(userID string) (*ConversationIndex, error) {
	indexPath := r.getIndexFile(userID)

	// Check if file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		// Return empty index
		return &ConversationIndex{
			Version:       "1.0",
			Conversations: make(map[string]ConversationIndexEntry),
		}, nil
	}

	// Read file
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file: %w", err)
	}

	// Parse JSON
	var index ConversationIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index file: %w", err)
	}

	// Initialize map if nil
	if index.Conversations == nil {
		index.Conversations = make(map[string]ConversationIndexEntry)
	}

	return &index, nil
}

// saveIndex saves the conversation index for a user
func (r *FileConversationRepository) saveIndex(userID string, index *ConversationIndex) error {
	indexPath := r.getIndexFile(userID)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// Update timestamp
	index.LastUpdated = time.Now().Format(time.RFC3339)

	// Marshal to JSON
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write atomically
	if err := r.writer.Write(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// loadMessages loads all messages from a conversation's JSONL file
func (r *FileConversationRepository) loadMessages(userID, convID string) ([]domain.StoredMessage, error) {
	messagesPath := r.getMessagesFile(userID, convID)

	// Check if file exists
	if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
		// Return empty slice
		return []domain.StoredMessage{}, nil
	}

	// Open file
	file, err := os.Open(messagesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open messages file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read messages line by line
	var messages []domain.StoredMessage
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var msg domain.StoredMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			return nil, fmt.Errorf("failed to parse message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read messages file: %w", err)
	}

	return messages, nil
}

// SaveConversation creates or updates a conversation
func (r *FileConversationRepository) SaveConversation(ctx context.Context, conv *domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure conversation directory exists
	convDir := r.getConvDir(conv.UserID, conv.ID)
	if err := os.MkdirAll(convDir, 0755); err != nil {
		return fmt.Errorf("failed to create conversation directory: %w", err)
	}

	// Write messages to JSONL file
	messagesPath := r.getMessagesFile(conv.UserID, conv.ID)
	file, err := os.Create(messagesPath)
	if err != nil {
		return fmt.Errorf("failed to create messages file: %w", err)
	}
	defer func() { _ = file.Close() }()

	writer := bufio.NewWriter(file)
	for _, msg := range conv.Messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		if _, err := writer.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush messages: %w", err)
	}

	// Update index
	index, err := r.loadIndex(conv.UserID)
	if err != nil {
		return err
	}

	lastSnippet := ""
	if len(conv.Messages) > 0 {
		lastMsg := conv.Messages[len(conv.Messages)-1]
		if len(lastMsg.Content) > 100 {
			lastSnippet = lastMsg.Content[:100] + "..."
		} else {
			lastSnippet = lastMsg.Content
		}
	}

	index.Conversations[conv.ID] = ConversationIndexEntry{
		ID:                 conv.ID,
		UserID:             conv.UserID,
		Platform:           conv.Platform,
		CreatedAt:          conv.CreatedAt,
		UpdatedAt:          conv.UpdatedAt,
		MessageCount:       len(conv.Messages),
		LastMessageSnippet: lastSnippet,
		FilePath:           filepath.Join(conv.ID, "messages.jsonl"),
	}

	return r.saveIndex(conv.UserID, index)
}

// findUserByConvID finds the user ID that owns a conversation
// Returns userID and entry if found, or error if not found
func (r *FileConversationRepository) findUserByConvID(convID string) (string, ConversationIndexEntry, error) {
	usersDir := filepath.Join(r.basePath, "users")
	users, err := os.ReadDir(usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ConversationIndexEntry{}, domain.ErrNotFound
		}
		return "", ConversationIndexEntry{}, fmt.Errorf("failed to read users directory: %w", err)
	}

	// Search through user indexes
	for _, user := range users {
		if !user.IsDir() {
			continue
		}
		userID := user.Name()
		index, err := r.loadIndex(userID)
		if err != nil {
			continue // Skip this user if index can't be loaded
		}

		if entry, found := index.Conversations[convID]; found {
			return userID, entry, nil
		}
	}

	return "", ConversationIndexEntry{}, domain.ErrNotFound
}

// GetConversation retrieves a conversation by ID
func (r *FileConversationRepository) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the user who owns this conversation
	userID, entry, err := r.findUserByConvID(convID)
	if err != nil {
		return nil, err
	}

	// Load messages
	messages, err := r.loadMessages(userID, convID)
	if err != nil {
		return nil, err
	}

	return &domain.Conversation{
		ID:        entry.ID,
		UserID:    entry.UserID,
		Platform:  entry.Platform,
		Messages:  messages,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
	}, nil
}

// ListConversations returns conversation summaries for a user
func (r *FileConversationRepository) ListConversations(ctx context.Context, userID string) ([]*domain.ConversationSummary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, err := r.loadIndex(userID)
	if err != nil {
		return nil, err
	}

	summaries := make([]*domain.ConversationSummary, 0, len(index.Conversations))
	for _, entry := range index.Conversations {
		summaries = append(summaries, &domain.ConversationSummary{
			ID:                 entry.ID,
			UserID:             entry.UserID,
			Platform:           entry.Platform,
			CreatedAt:          entry.CreatedAt,
			UpdatedAt:          entry.UpdatedAt,
			LastMessageSnippet: entry.LastMessageSnippet,
			MessageCount:       entry.MessageCount,
		})
	}

	return summaries, nil
}

// DeleteConversation removes a conversation by ID
func (r *FileConversationRepository) DeleteConversation(ctx context.Context, convID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the user who owns this conversation
	userID, _, err := r.findUserByConvID(convID)
	if err != nil {
		return err
	}

	// Delete conversation directory
	convDir := r.getConvDir(userID, convID)
	if err := os.RemoveAll(convDir); err != nil {
		return fmt.Errorf("failed to delete conversation directory: %w", err)
	}

	// Remove from index
	index, err := r.loadIndex(userID)
	if err != nil {
		return err
	}
	delete(index.Conversations, convID)
	return r.saveIndex(userID, index)
}

// AppendMessage adds a message to an existing conversation
func (r *FileConversationRepository) AppendMessage(ctx context.Context, convID string, message domain.StoredMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the user who owns this conversation
	userID, entry, err := r.findUserByConvID(convID)
	if err != nil {
		return err
	}

	// Append message to JSONL file
	messagesPath := r.getMessagesFile(userID, convID)
	file, err := os.OpenFile(messagesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open messages file: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Update index
	index, err := r.loadIndex(userID)
	if err != nil {
		return err
	}
	entry.UpdatedAt = time.Now()
	entry.MessageCount++
	if len(message.Content) > 100 {
		entry.LastMessageSnippet = message.Content[:100] + "..."
	} else {
		entry.LastMessageSnippet = message.Content
	}
	index.Conversations[convID] = entry
	return r.saveIndex(userID, index)
}

// CountMessages returns the total number of messages in a conversation
func (r *FileConversationRepository) CountMessages(ctx context.Context, convID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the user who owns this conversation
	_, entry, err := r.findUserByConvID(convID)
	if err != nil {
		return 0, err
	}

	return entry.MessageCount, nil
}
