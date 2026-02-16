package persona

import (
	"context"
	"time"

	"nuimanbot/internal/domain"
)

// mockRepository implements domain.PersonaFileRepository for testing.
type mockRepository struct {
	files map[string]map[domain.PersonaFileType]*domain.PersonaFile
	err   error
}

func newMockRepo() *mockRepository {
	return &mockRepository{
		files: make(map[string]map[domain.PersonaFileType]*domain.PersonaFile),
	}
}

func (m *mockRepository) Get(_ context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	userFiles, ok := m.files[userID]
	if !ok {
		return nil, domain.ErrPersonaFileNotFound
	}
	file, ok := userFiles[fileType]
	if !ok {
		return nil, domain.ErrPersonaFileNotFound
	}
	return file, nil
}

func (m *mockRepository) Save(_ context.Context, file *domain.PersonaFile) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.files[file.UserID]; !ok {
		m.files[file.UserID] = make(map[domain.PersonaFileType]*domain.PersonaFile)
	}
	m.files[file.UserID][file.Type] = file
	return nil
}

func (m *mockRepository) Delete(_ context.Context, userID string, fileType domain.PersonaFileType) error {
	if m.err != nil {
		return m.err
	}
	if userFiles, ok := m.files[userID]; ok {
		delete(userFiles, fileType)
	}
	return nil
}

func (m *mockRepository) List(_ context.Context, userID string) ([]*domain.PersonaFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	userFiles, ok := m.files[userID]
	if !ok {
		return nil, nil
	}
	var result []*domain.PersonaFile
	for _, ft := range []domain.PersonaFileType{domain.PersonaFileSOUL, domain.PersonaFileUSER, domain.PersonaFileRULES} {
		if f, ok := userFiles[ft]; ok {
			result = append(result, f)
		}
	}
	return result, nil
}

func (m *mockRepository) addFile(userID string, fileType domain.PersonaFileType, content string) {
	if m.files[userID] == nil {
		m.files[userID] = make(map[domain.PersonaFileType]*domain.PersonaFile)
	}
	m.files[userID][fileType] = &domain.PersonaFile{
		UserID:     userID,
		Type:       fileType,
		Path:       "/data/" + userID + "/" + fileType.Filename(),
		Content:    content,
		ModifiedAt: time.Now(),
		SizeBytes:  int64(len(content)),
	}
}
