package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nuimanbot/internal/domain"
)

// SQLiteMemoryStorage implements skill memory using SQLite
type SQLiteMemoryStorage struct {
	db *sql.DB
}

// NewSQLiteMemoryStorage creates a new SQLite memory storage
func NewSQLiteMemoryStorage(dbPath string) (*SQLiteMemoryStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &SQLiteMemoryStorage{db: db}

	if err := storage.createSchema(); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return storage, nil
}

// createSchema creates the memory table
func (s *SQLiteMemoryStorage) createSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS skill_memory (
		id TEXT PRIMARY KEY,
		skill_name TEXT NOT NULL,
		scope TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP,
		metadata TEXT,
		UNIQUE(skill_name, scope, key)
	);
	CREATE INDEX IF NOT EXISTS idx_skill_scope ON skill_memory(skill_name, scope);
	CREATE INDEX IF NOT EXISTS idx_expires ON skill_memory(expires_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Get retrieves a memory value
func (s *SQLiteMemoryStorage) Get(skillName, key string, scope domain.MemoryScope) (*domain.SkillMemory, error) {
	query := `SELECT id, skill_name, scope, key, value, created_at, updated_at, expires_at, metadata
	          FROM skill_memory WHERE skill_name = ? AND key = ? AND scope = ?`

	row := s.db.QueryRow(query, skillName, key, scope)

	var memory domain.SkillMemory
	var expiresAt sql.NullTime
	var metadataJSON sql.NullString

	err := row.Scan(
		&memory.ID,
		&memory.SkillName,
		&memory.Scope,
		&memory.Key,
		&memory.Value,
		&memory.CreatedAt,
		&memory.UpdatedAt,
		&expiresAt,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory not found")
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		memory.ExpiresAt = &expiresAt.Time
	}

	if metadataJSON.Valid {
		json.Unmarshal([]byte(metadataJSON.String), &memory.Metadata)
	}

	return &memory, nil
}

// Set stores a memory value
func (s *SQLiteMemoryStorage) Set(memory *domain.SkillMemory) error {
	if err := memory.Validate(); err != nil {
		return err
	}

	if memory.ID == "" {
		memory.ID = fmt.Sprintf("%s-%s-%s-%d", memory.SkillName, memory.Scope, memory.Key, time.Now().UnixNano())
	}

	now := time.Now()
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	memory.UpdatedAt = now

	var metadataJSON []byte
	if memory.Metadata != nil {
		metadataJSON, _ = json.Marshal(memory.Metadata)
	}

	query := `INSERT OR REPLACE INTO skill_memory
	          (id, skill_name, scope, key, value, created_at, updated_at, expires_at, metadata)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		memory.ID,
		memory.SkillName,
		memory.Scope,
		memory.Key,
		memory.Value,
		memory.CreatedAt,
		memory.UpdatedAt,
		memory.ExpiresAt,
		metadataJSON,
	)

	return err
}

// Delete removes a memory value
func (s *SQLiteMemoryStorage) Delete(skillName, key string, scope domain.MemoryScope) error {
	query := `DELETE FROM skill_memory WHERE skill_name = ? AND key = ? AND scope = ?`
	_, err := s.db.Exec(query, skillName, key, scope)
	return err
}

// List lists all memory for a skill
func (s *SQLiteMemoryStorage) List(skillName string, scope domain.MemoryScope) ([]*domain.SkillMemory, error) {
	query := `SELECT id, skill_name, scope, key, value, created_at, updated_at, expires_at, metadata
	          FROM skill_memory WHERE skill_name = ? AND scope = ?`

	rows, err := s.db.Query(query, skillName, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*domain.SkillMemory

	for rows.Next() {
		var memory domain.SkillMemory
		var expiresAt sql.NullTime
		var metadataJSON sql.NullString

		err := rows.Scan(
			&memory.ID,
			&memory.SkillName,
			&memory.Scope,
			&memory.Key,
			&memory.Value,
			&memory.CreatedAt,
			&memory.UpdatedAt,
			&expiresAt,
			&metadataJSON,
		)

		if err != nil {
			continue
		}

		if expiresAt.Valid {
			memory.ExpiresAt = &expiresAt.Time
		}

		if metadataJSON.Valid {
			json.Unmarshal([]byte(metadataJSON.String), &memory.Metadata)
		}

		memories = append(memories, &memory)
	}

	return memories, nil
}

// Cleanup removes expired memory
func (s *SQLiteMemoryStorage) Cleanup() error {
	query := `DELETE FROM skill_memory WHERE expires_at IS NOT NULL AND expires_at < ?`
	_, err := s.db.Exec(query, time.Now())
	return err
}

// Close closes the database connection
func (s *SQLiteMemoryStorage) Close() error {
	return s.db.Close()
}
