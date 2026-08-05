package main

import (
	"log/slog"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
)

// fileStorageRepositories holds all file-based repository implementations
type fileStorageRepositories struct {
	UserProfile     domain.UserProfileRepository
	Conversation    domain.ConversationRepository
	Notes           domain.NotesRepository
	Audit           domain.AuditRepository
	MemoryCell      memoryv2.MemoryCellRepository
	MemoryScene     memoryv2.MemorySceneRepository
	MemoryCellFile  *storage.FileMemoryCellRepository  // Concrete type for admin operations
	MemorySceneFile *storage.FileMemorySceneRepository // Concrete type for admin operations

	// Project/Job/Chore/Run back the web admin's extended-context
	// environments (specs/260805-nuimanbot-extend-context-and-ui).
	Project domain.ProjectRepository
	Job     domain.JobRepository
	Chore   domain.ChoreRepository
	Run     domain.RunRepository
}

// initializeFileStorage creates all file-based repositories
func initializeFileStorage(basePath, encryptionKey string) (*fileStorageRepositories, error) {
	slog.Info("Initializing file-based storage",
		"base_path", basePath,
	)

	cellRepo := storage.NewFileMemoryCellRepository(basePath)
	sceneRepo := storage.NewFileMemorySceneRepository(basePath)

	repos := &fileStorageRepositories{
		UserProfile:     storage.NewFileUserProfileRepository(basePath+"/users.json", encryptionKey),
		Conversation:    storage.NewFileConversationRepository(basePath),
		Notes:           storage.NewFileNotesRepository(basePath),
		Audit:           storage.NewFileAuditRepository(basePath),
		MemoryCell:      cellRepo,
		MemoryScene:     sceneRepo,
		MemoryCellFile:  cellRepo,
		MemorySceneFile: sceneRepo,
		Project:         storage.NewFileProjectRepository(basePath),
		Job:             storage.NewFileJobRepository(basePath),
		Chore:           storage.NewFileChoreRepository(basePath),
		Run:             storage.NewFileRunRepository(basePath),
	}

	slog.Info("File-based storage initialized successfully",
		"repositories", 10,
	)

	return repos, nil
}
