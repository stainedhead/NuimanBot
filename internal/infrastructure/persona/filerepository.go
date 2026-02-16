package persona

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nuimanbot/internal/domain"
)

// FileCache caches file content to reduce disk I/O.
type FileCache struct {
	ttl     time.Duration
	entries map[string]*CacheEntry
	mu      sync.RWMutex
}

// CacheEntry represents a cached file with metadata.
type CacheEntry struct {
	Content    string
	ModifiedAt time.Time
	SizeBytes  int64
	ExpiresAt  time.Time
}

// NewFileCache creates a new FileCache with the given TTL.
func NewFileCache(ttl time.Duration) *FileCache {
	return &FileCache{
		ttl:     ttl,
		entries: make(map[string]*CacheEntry),
	}
}

// Get retrieves a cached entry. Returns nil and false if not found or expired.
func (c *FileCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, true
}

// Set stores a value in the cache with file metadata.
func (c *FileCache) Set(key string, content string, modifiedAt time.Time, sizeBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Content:    content,
		ModifiedAt: modifiedAt,
		SizeBytes:  sizeBytes,
		ExpiresAt:  time.Now().Add(c.ttl),
	}
}

// Delete removes a single cache entry.
func (c *FileCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// DeletePrefix removes all entries whose key starts with the given prefix.
func (c *FileCache) DeletePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// FileRepository implements domain.PersonaFileRepository using the filesystem.
type FileRepository struct {
	basePath string
	cache    *FileCache
}

// NewFileRepository creates a new FileRepository rooted at basePath.
func NewFileRepository(basePath string) *FileRepository {
	return &FileRepository{
		basePath: basePath,
		cache:    NewFileCache(15 * time.Minute),
	}
}

// cacheKey returns the cache key for a user/type combination.
func cacheKey(userID string, fileType domain.PersonaFileType) string {
	return userID + ":" + fileType.String()
}

// resolvePath builds and validates the filesystem path for a persona file.
func (r *FileRepository) resolvePath(userID string, fileType domain.PersonaFileType) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("userID is required")
	}
	if !fileType.IsValid() {
		return "", domain.ErrInvalidPersonaFileType
	}

	return ValidateUserPath(r.basePath, userID, fileType.Filename())
}

// Save creates or updates a persona file.
func (r *FileRepository) Save(ctx context.Context, file *domain.PersonaFile) error {
	// Always resolve and validate the path (protects against path traversal)
	validatedPath, err := r.resolvePath(file.UserID, file.Type)
	if err != nil {
		return err
	}

	// If Path is not set, use the validated path
	if file.Path == "" {
		file.Path = validatedPath
	} else {
		// If Path is set, verify it matches the expected validated path
		// This prevents callers from bypassing path validation
		if file.Path != validatedPath {
			return fmt.Errorf("path mismatch: expected %s, got %s", validatedPath, file.Path)
		}
	}

	// Set ModifiedAt to now if not set
	if file.ModifiedAt.IsZero() {
		file.ModifiedAt = time.Now()
	}

	if err := file.Validate(); err != nil {
		return err
	}

	absPath := file.Path

	// Ensure user directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating user directory: %w", err)
	}

	// Security: Remove existing file if it's a symlink before writing
	// This prevents users from creating symlinks and then updating them
	if err := ValidateNoSymlink(absPath); err != nil {
		if errors.Is(err, domain.ErrPathTraversal) {
			// File exists and is a symlink - remove it
			if rmErr := os.Remove(absPath); rmErr != nil {
				return fmt.Errorf("removing symlink: %w", rmErr)
			}
		}
		// If error is something else (like permission denied), let WriteFile handle it
	}

	if err := os.WriteFile(absPath, []byte(file.Content), 0644); err != nil {
		return fmt.Errorf("writing persona file: %w", err)
	}

	// Invalidate cache
	r.cache.Delete(cacheKey(file.UserID, file.Type))

	return nil
}

// Get retrieves a persona file by user ID and type.
func (r *FileRepository) Get(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
	absPath, err := r.resolvePath(userID, fileType)
	if err != nil {
		if errors.Is(err, domain.ErrPathTraversal) {
			return nil, domain.ErrPathTraversal
		}
		return nil, err
	}

	// Check cache
	key := cacheKey(userID, fileType)
	if entry, ok := r.cache.Get(key); ok {
		return &domain.PersonaFile{
			UserID:     userID,
			Type:       fileType,
			Path:       absPath,
			Content:    entry.Content,
			ModifiedAt: entry.ModifiedAt,
			SizeBytes:  entry.SizeBytes,
		}, nil
	}

	// Security: Block symlinks to prevent reading files outside user directory
	if err := ValidateNoSymlink(absPath); err != nil {
		if errors.Is(err, domain.ErrPathTraversal) {
			return nil, domain.ErrPathTraversal
		}
		return nil, fmt.Errorf("symlink validation failed: %w", err)
	}

	// Read from disk
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrPersonaFileNotFound
		}
		return nil, fmt.Errorf("reading persona file: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("stat persona file: %w", err)
	}

	content := string(data)

	// Populate cache with content and metadata
	r.cache.Set(key, content, info.ModTime(), info.Size())

	return &domain.PersonaFile{
		UserID:     userID,
		Type:       fileType,
		Path:       absPath,
		Content:    content,
		ModifiedAt: info.ModTime(),
		SizeBytes:  info.Size(),
	}, nil
}

// Delete removes a persona file. Returns nil if file doesn't exist (idempotent).
func (r *FileRepository) Delete(ctx context.Context, userID string, fileType domain.PersonaFileType) error {
	absPath, err := r.resolvePath(userID, fileType)
	if err != nil {
		if errors.Is(err, domain.ErrPathTraversal) {
			return domain.ErrPathTraversal
		}
		return err
	}

	// Invalidate cache
	r.cache.Delete(cacheKey(userID, fileType))

	if err := os.Remove(absPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Idempotent
		}
		return fmt.Errorf("deleting persona file: %w", err)
	}

	return nil
}

// List returns all persona files for a user.
func (r *FileRepository) List(ctx context.Context, userID string) ([]*domain.PersonaFile, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	var files []*domain.PersonaFile
	for _, ft := range []domain.PersonaFileType{domain.PersonaFileSOUL, domain.PersonaFileUSER, domain.PersonaFileRULES} {
		file, err := r.Get(ctx, userID, ft)
		if err != nil {
			if errors.Is(err, domain.ErrPersonaFileNotFound) {
				continue
			}
			return nil, err
		}
		files = append(files, file)
	}

	return files, nil
}
