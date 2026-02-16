package main

import (
	"log/slog"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
)

// fileStorageRepositories holds all file-based repository implementations
type fileStorageRepositories struct {
	UserProfile  domain.UserProfileRepository
	Conversation domain.ConversationRepository
	Notes        domain.NotesRepository
	Audit        domain.AuditRepository
	MemoryCell   memoryv2.MemoryCellRepository
	MemoryScene  memoryv2.MemorySceneRepository
}

// initializeFileStorage creates all file-based repositories
func initializeFileStorage(basePath, encryptionKey string) (*fileStorageRepositories, error) {
	slog.Info("Initializing file-based storage",
		"base_path", basePath,
	)

	repos := &fileStorageRepositories{
		UserProfile:  storage.NewFileUserProfileRepository(basePath+"/users.json", encryptionKey),
		Conversation: storage.NewFileConversationRepository(basePath),
		Notes:        storage.NewFileNotesRepository(basePath),
		Audit:        storage.NewFileAuditRepository(basePath),
		MemoryCell:   storage.NewFileMemoryCellRepository(basePath),
		MemoryScene:  storage.NewFileMemorySceneRepository(basePath),
	}

	slog.Info("File-based storage initialized successfully",
		"repositories", 6,
	)

	return repos, nil
}
