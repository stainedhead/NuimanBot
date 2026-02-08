package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/security"
	"os"
	"sync"
)

// BotConfigsFile represents the structure of bots.json
type BotConfigsFile struct {
	Version      string                      `json:"version"`
	LastUpdated  string                      `json:"lastUpdated"`
	SlackBots    []*domain.SlackBotConfig    `json:"slackBots"`
	TelegramBots []*domain.TelegramBotConfig `json:"telegramBots"`
	Indexes      BotConfigIndexes            `json:"indexes"`
}

// BotConfigIndexes contains lookup indexes for fast retrieval
type BotConfigIndexes struct {
	SlackByName        map[string]string   `json:"slackByName"`        // name -> botID
	SlackByOwner       map[string][]string `json:"slackByOwner"`       // ownerID -> []botID
	SlackByBotUserID   map[string]string   `json:"slackByBotUserID"`   // slackBotUserID -> botID
	TelegramByName     map[string]string   `json:"telegramByName"`     // name -> botID
	TelegramByOwner    map[string][]string `json:"telegramByOwner"`    // ownerID -> []botID
	TelegramByBotID    map[string]string   `json:"telegramByBotID"`    // telegramBotID -> botID
	TelegramByUsername map[string]string   `json:"telegramByUsername"` // username -> botID
}

// FileBotConfigRepository implements BotConfigRepository using JSON file storage with encryption
type FileBotConfigRepository struct {
	filePath   string
	writer     *AtomicFileWriter
	encryption *security.EncryptionService
	mu         sync.RWMutex
}

// NewFileBotConfigRepository creates a new file-based bot configuration repository
func NewFileBotConfigRepository(filePath string, encryption *security.EncryptionService) *FileBotConfigRepository {
	return &FileBotConfigRepository{
		filePath:   filePath,
		writer:     NewAtomicFileWriter(),
		encryption: encryption,
	}
}

// load reads and parses the bots.json file
func (r *FileBotConfigRepository) load() (*BotConfigsFile, error) {
	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		// Return empty file structure
		return r.emptyFile(), nil
	}

	// Read file
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bots file: %w", err)
	}

	// Parse JSON
	var file BotConfigsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse bots file: %w", err)
	}

	// Initialize maps if nil
	r.initializeIndexes(&file)

	// Initialize slices if nil
	if file.SlackBots == nil {
		file.SlackBots = []*domain.SlackBotConfig{}
	}
	if file.TelegramBots == nil {
		file.TelegramBots = []*domain.TelegramBotConfig{}
	}

	return &file, nil
}

// emptyFile creates an empty bots file structure
func (r *FileBotConfigRepository) emptyFile() *BotConfigsFile {
	file := &BotConfigsFile{
		Version:      "1.0",
		SlackBots:    []*domain.SlackBotConfig{},
		TelegramBots: []*domain.TelegramBotConfig{},
	}
	r.initializeIndexes(file)
	return file
}

// initializeIndexes ensures all index maps are initialized
func (r *FileBotConfigRepository) initializeIndexes(file *BotConfigsFile) {
	if file.Indexes.SlackByName == nil {
		file.Indexes.SlackByName = make(map[string]string)
	}
	if file.Indexes.SlackByOwner == nil {
		file.Indexes.SlackByOwner = make(map[string][]string)
	}
	if file.Indexes.SlackByBotUserID == nil {
		file.Indexes.SlackByBotUserID = make(map[string]string)
	}
	if file.Indexes.TelegramByName == nil {
		file.Indexes.TelegramByName = make(map[string]string)
	}
	if file.Indexes.TelegramByOwner == nil {
		file.Indexes.TelegramByOwner = make(map[string][]string)
	}
	if file.Indexes.TelegramByBotID == nil {
		file.Indexes.TelegramByBotID = make(map[string]string)
	}
	if file.Indexes.TelegramByUsername == nil {
		file.Indexes.TelegramByUsername = make(map[string]string)
	}
}

// save writes the bots file atomically
func (r *FileBotConfigRepository) save(file *BotConfigsFile) error {
	// Update timestamp
	file.LastUpdated = formatTimestamp()

	// Make a deep copy to avoid modifying the original
	fileCopy := r.deepCopyFile(file)

	// Encrypt bot tokens before saving
	if err := r.encryptTokens(fileCopy); err != nil {
		return fmt.Errorf("failed to encrypt tokens: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(fileCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bots file: %w", err)
	}

	// Write atomically
	if err := r.writer.Write(r.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write bots file: %w", err)
	}

	return nil
}

// deepCopyFile creates a deep copy of the bots file to avoid modifying originals during encryption.
// Uses JSON marshaling as a simple deep copy mechanism.
// Errors are ignored here because the file structure is already validated.
func (r *FileBotConfigRepository) deepCopyFile(file *BotConfigsFile) *BotConfigsFile {
	// Marshal and unmarshal to create a deep copy
	data, err := json.Marshal(file)
	if err != nil {
		// This should never happen with valid data, but return original if it does
		return file
	}

	var copy BotConfigsFile
	if err := json.Unmarshal(data, &copy); err != nil {
		// This should never happen with valid JSON, but return original if it does
		return file
	}

	return &copy
}

// encryptTokens encrypts all bot tokens in the file
func (r *FileBotConfigRepository) encryptTokens(file *BotConfigsFile) error {
	// Encrypt Slack bot tokens
	for _, bot := range file.SlackBots {
		if err := r.encryptString(&bot.SlackBotToken, "Slack bot token"); err != nil {
			return err
		}
		if err := r.encryptString(&bot.SlackAppToken, "Slack app token"); err != nil {
			return err
		}
		if err := r.encryptString(&bot.SlackSigningSecret, "Slack signing secret"); err != nil {
			return err
		}
	}

	// Encrypt Telegram bot tokens
	for _, bot := range file.TelegramBots {
		if err := r.encryptString(&bot.TelegramBotToken, "Telegram bot token"); err != nil {
			return err
		}
	}

	return nil
}

// decryptTokens decrypts all bot tokens in the file
func (r *FileBotConfigRepository) decryptTokens(file *BotConfigsFile) error {
	// Decrypt Slack bot tokens
	for _, bot := range file.SlackBots {
		if err := r.decryptString(&bot.SlackBotToken, "Slack bot token"); err != nil {
			return err
		}
		if err := r.decryptString(&bot.SlackAppToken, "Slack app token"); err != nil {
			return err
		}
		if err := r.decryptString(&bot.SlackSigningSecret, "Slack signing secret"); err != nil {
			return err
		}
	}

	// Decrypt Telegram bot tokens
	for _, bot := range file.TelegramBots {
		if err := r.decryptString(&bot.TelegramBotToken, "Telegram bot token"); err != nil {
			return err
		}
	}

	return nil
}

// encryptString encrypts a string value if it's not already encrypted
func (r *FileBotConfigRepository) encryptString(value *string, fieldName string) error {
	if *value == "" || r.isEncrypted(*value) {
		return nil
	}

	encrypted, err := r.encryption.Encrypt(*value)
	if err != nil {
		return fmt.Errorf("failed to encrypt %s: %w", fieldName, err)
	}

	*value = encrypted
	return nil
}

// decryptString decrypts a string value if it's encrypted
func (r *FileBotConfigRepository) decryptString(value *string, fieldName string) error {
	if *value == "" || !r.isEncrypted(*value) {
		return nil
	}

	decrypted, err := r.encryption.Decrypt(*value)
	if err != nil {
		return fmt.Errorf("failed to decrypt %s: %w", fieldName, err)
	}

	*value = decrypted
	return nil
}

// isEncrypted checks if a string looks like an encrypted value (base64)
// A simple heuristic: encrypted values are typically longer and contain base64 characters
func (r *FileBotConfigRepository) isEncrypted(value string) bool {
	// Encrypted values from AES-GCM are base64 encoded and typically > 40 chars
	return len(value) > 40 && isBase64(value)
}

// isBase64 checks if a string appears to be base64 encoded
func isBase64(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Base64 uses A-Za-z0-9+/= characters
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=') {
			return false
		}
	}
	return true
}

// rebuildIndexes rebuilds all indexes from the bot lists
func (r *FileBotConfigRepository) rebuildIndexes(file *BotConfigsFile) {
	// Reset all indexes
	file.Indexes.SlackByName = make(map[string]string)
	file.Indexes.SlackByOwner = make(map[string][]string)
	file.Indexes.SlackByBotUserID = make(map[string]string)
	file.Indexes.TelegramByName = make(map[string]string)
	file.Indexes.TelegramByOwner = make(map[string][]string)
	file.Indexes.TelegramByBotID = make(map[string]string)
	file.Indexes.TelegramByUsername = make(map[string]string)

	// Build Slack indexes
	for _, bot := range file.SlackBots {
		// Index by name
		if bot.BotName != "" {
			file.Indexes.SlackByName[bot.BotName] = bot.BotID
		}

		// Index by owner
		if bot.OwnerUserID != "" {
			file.Indexes.SlackByOwner[bot.OwnerUserID] = append(
				file.Indexes.SlackByOwner[bot.OwnerUserID],
				bot.BotID,
			)
		}

		// Index by Slack bot user ID
		if bot.SlackBotUserID != "" {
			file.Indexes.SlackByBotUserID[bot.SlackBotUserID] = bot.BotID
		}
	}

	// Build Telegram indexes
	for _, bot := range file.TelegramBots {
		// Index by name
		if bot.BotName != "" {
			file.Indexes.TelegramByName[bot.BotName] = bot.BotID
		}

		// Index by owner
		if bot.OwnerUserID != "" {
			file.Indexes.TelegramByOwner[bot.OwnerUserID] = append(
				file.Indexes.TelegramByOwner[bot.OwnerUserID],
				bot.BotID,
			)
		}

		// Index by Telegram bot ID
		if bot.TelegramBotID != "" {
			file.Indexes.TelegramByBotID[bot.TelegramBotID] = bot.BotID
		}

		// Index by username
		if bot.TelegramBotUsername != "" {
			file.Indexes.TelegramByUsername[bot.TelegramBotUsername] = bot.BotID
		}
	}
}

// Slack Bot Operations

// SaveSlackBot creates or updates a Slack bot configuration
func (r *FileBotConfigRepository) SaveSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load current file
	file, err := r.load()
	if err != nil {
		return err
	}

	// Find existing bot
	found := false
	for i, existing := range file.SlackBots {
		if existing.BotID == bot.BotID {
			file.SlackBots[i] = bot
			found = true
			break
		}
	}

	// Add new bot if not found
	if !found {
		file.SlackBots = append(file.SlackBots, bot)
	}

	// Rebuild indexes
	r.rebuildIndexes(file)

	// Save file
	return r.save(file)
}

// GetSlackBotByID retrieves a Slack bot by ID
func (r *FileBotConfigRepository) GetSlackBotByID(ctx context.Context, botID string) (*domain.SlackBotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Load file
	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Decrypt tokens
	if err := r.decryptTokens(file); err != nil {
		return nil, err
	}

	// Find bot
	for _, bot := range file.SlackBots {
		if bot.BotID == botID {
			return bot, nil
		}
	}

	return nil, errors.New("bot not found")
}

// ListSlackBots returns all Slack bots
func (r *FileBotConfigRepository) ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Load file
	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Decrypt tokens
	if err := r.decryptTokens(file); err != nil {
		return nil, err
	}

	return file.SlackBots, nil
}

// ListSlackBotsByOwner returns all Slack bots owned by a specific user
func (r *FileBotConfigRepository) ListSlackBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.SlackBotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Load file
	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Get bot IDs from index
	botIDs, ok := file.Indexes.SlackByOwner[ownerUserID]
	if !ok || len(botIDs) == 0 {
		return []*domain.SlackBotConfig{}, nil
	}

	// Collect bots by ID
	result := make([]*domain.SlackBotConfig, 0, len(botIDs))
	for _, bot := range file.SlackBots {
		for _, botID := range botIDs {
			if bot.BotID == botID {
				result = append(result, bot)
				break
			}
		}
	}

	// Decrypt tokens
	if err := r.decryptTokens(file); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteSlackBot removes a Slack bot by ID
func (r *FileBotConfigRepository) DeleteSlackBot(ctx context.Context, botID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load current file
	file, err := r.load()
	if err != nil {
		return err
	}

	// Find and remove bot
	found := false
	for i, bot := range file.SlackBots {
		if bot.BotID == botID {
			// Remove bot from slice
			file.SlackBots = append(file.SlackBots[:i], file.SlackBots[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return errors.New("bot not found")
	}

	// Rebuild indexes
	r.rebuildIndexes(file)

	// Save file
	return r.save(file)
}

// Telegram Bot Operations

// SaveTelegramBot creates or updates a Telegram bot configuration
func (r *FileBotConfigRepository) SaveTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load current file
	file, err := r.load()
	if err != nil {
		return err
	}

	// Find existing bot
	found := false
	for i, existing := range file.TelegramBots {
		if existing.BotID == bot.BotID {
			file.TelegramBots[i] = bot
			found = true
			break
		}
	}

	// Add new bot if not found
	if !found {
		file.TelegramBots = append(file.TelegramBots, bot)
	}

	// Rebuild indexes
	r.rebuildIndexes(file)

	// Save file
	return r.save(file)
}

// GetTelegramBotByID retrieves a Telegram bot by ID
func (r *FileBotConfigRepository) GetTelegramBotByID(ctx context.Context, botID string) (*domain.TelegramBotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Load file
	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Decrypt tokens
	if err := r.decryptTokens(file); err != nil {
		return nil, err
	}

	// Find bot
	for _, bot := range file.TelegramBots {
		if bot.BotID == botID {
			return bot, nil
		}
	}

	return nil, errors.New("bot not found")
}

// ListTelegramBots returns all Telegram bots
func (r *FileBotConfigRepository) ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Load file
	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Decrypt tokens
	if err := r.decryptTokens(file); err != nil {
		return nil, err
	}

	return file.TelegramBots, nil
}

// ListTelegramBotsByOwner returns all Telegram bots owned by a specific user
func (r *FileBotConfigRepository) ListTelegramBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.TelegramBotConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Load file
	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Get bot IDs from index
	botIDs, ok := file.Indexes.TelegramByOwner[ownerUserID]
	if !ok || len(botIDs) == 0 {
		return []*domain.TelegramBotConfig{}, nil
	}

	// Collect bots by ID
	result := make([]*domain.TelegramBotConfig, 0, len(botIDs))
	for _, bot := range file.TelegramBots {
		for _, botID := range botIDs {
			if bot.BotID == botID {
				result = append(result, bot)
				break
			}
		}
	}

	// Decrypt tokens
	if err := r.decryptTokens(file); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteTelegramBot removes a Telegram bot by ID
func (r *FileBotConfigRepository) DeleteTelegramBot(ctx context.Context, botID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load current file
	file, err := r.load()
	if err != nil {
		return err
	}

	// Find and remove bot
	found := false
	for i, bot := range file.TelegramBots {
		if bot.BotID == botID {
			// Remove bot from slice
			file.TelegramBots = append(file.TelegramBots[:i], file.TelegramBots[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return errors.New("bot not found")
	}

	// Rebuild indexes
	r.rebuildIndexes(file)

	// Save file
	return r.save(file)
}
