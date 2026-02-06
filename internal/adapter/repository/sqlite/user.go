package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"nuimanbot/internal/domain"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// UserRepository implements domain.UserRepository for SQLite.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new SQLite user repository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Init initializes the user table if it doesn't exist.
func (r *UserRepository) Init(ctx context.Context) error {
	const createTableSQL = `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		role TEXT NOT NULL,
		platform_ids TEXT NOT NULL, -- Stored as JSON
		allowed_skills TEXT NOT NULL, -- Stored as JSON
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`
	_, err := r.db.ExecContext(ctx, createTableSQL)
	return err
}

// SaveUser creates or updates a user in the database.
func (r *UserRepository) SaveUser(ctx context.Context, user *domain.User) error {
	platformIDsJSON, err := json.Marshal(user.PlatformIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal platform IDs: %w", err)
	}
	allowedSkillsJSON, err := json.Marshal(user.AllowedSkills)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed skills: %w", err)
	}

	user.UpdatedAt = time.Now() // Update timestamp on save

	const upsertSQL = `
	INSERT INTO users (id, username, role, platform_ids, allowed_skills, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		username = EXCLUDED.username,
		role = EXCLUDED.role,
		platform_ids = EXCLUDED.platform_ids,
		allowed_skills = EXCLUDED.allowed_skills,
		updated_at = EXCLUDED.updated_at;`

	_, err = r.db.ExecContext(
		ctx,
		upsertSQL,
		user.ID,
		user.Username,
		string(user.Role),
		platformIDsJSON,
		allowedSkillsJSON,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by their ID.
func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	const selectSQL = `SELECT id, username, role, platform_ids, allowed_skills, created_at, updated_at FROM users WHERE id = ?;`
	row := r.db.QueryRowContext(ctx, selectSQL, id)

	user := &domain.User{}
	var roleStr string
	var platformIDsJSON, allowedSkillsJSON []byte

	err := row.Scan(
		&user.ID,
		&user.Username,
		&roleStr,
		&platformIDsJSON,
		&allowedSkillsJSON,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound // Assuming ErrUserNotFound is defined in domain
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	user.Role = domain.Role(roleStr)

	err = json.Unmarshal(platformIDsJSON, &user.PlatformIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal platform IDs: %w", err)
	}
	err = json.Unmarshal(allowedSkillsJSON, &user.AllowedSkills)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal allowed skills: %w", err)
	}

	return user, nil
}

// GetUserByPlatformID retrieves a user by their platform ID.
func (r *UserRepository) GetUserByPlatformID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	// This query uses LIKE to search within the JSON 'platform_ids' column.
	// This is not the most efficient way to query JSON, but sufficient for MVP.
	// For production, consider a JSON-enabled database or a more robust solution.
	searchString := fmt.Sprintf(`"\"%q\":\"%q\""`, platform, platformUID)
	const selectSQL = `
	SELECT id, username, role, platform_ids, allowed_skills, created_at, updated_at
	FROM users
	WHERE platform_ids LIKE ?;`
	row := r.db.QueryRowContext(ctx, selectSQL, "%"+searchString+"%")

	user := &domain.User{}
	var roleStr string
	var platformIDsJSON, allowedSkillsJSON []byte

	err := row.Scan(
		&user.ID,
		&user.Username,
		&roleStr,
		&platformIDsJSON,
		&allowedSkillsJSON,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan user by platform ID: %w", err)
	}

	user.Role = domain.Role(roleStr)

	err = json.Unmarshal(platformIDsJSON, &user.PlatformIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal platform IDs: %w", err)
	}
	err = json.Unmarshal(allowedSkillsJSON, &user.AllowedSkills)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal allowed skills: %w", err)
	}

	return user, nil
}
