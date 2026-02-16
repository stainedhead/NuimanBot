package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain"
)

// MockPersonaFileRepository for testing
type MockPersonaFileRepository struct {
	SaveFunc func(ctx context.Context, file *domain.PersonaFile) error
	GetFunc  func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error)
}

func (m *MockPersonaFileRepository) Get(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, userID, fileType)
	}
	return nil, domain.ErrPersonaFileNotFound
}

func (m *MockPersonaFileRepository) Save(ctx context.Context, file *domain.PersonaFile) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, file)
	}
	return nil
}

func (m *MockPersonaFileRepository) Delete(ctx context.Context, userID string, fileType domain.PersonaFileType) error {
	return nil
}

func (m *MockPersonaFileRepository) List(ctx context.Context, userID string) ([]*domain.PersonaFile, error) {
	return nil, nil
}

// MockTemplateLoader for testing
type MockTemplateLoader struct {
	LoadFunc func(fileType domain.PersonaFileType) (string, error)
}

func (m *MockTemplateLoader) Load(fileType domain.PersonaFileType) (string, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc(fileType)
	}
	return "template content", nil
}

func TestPersonaCommand_Init(t *testing.T) {
	savedFiles := make(map[domain.PersonaFileType]string)

	mockRepo := &MockPersonaFileRepository{
		SaveFunc: func(ctx context.Context, file *domain.PersonaFile) error {
			savedFiles[file.Type] = file.Content
			return nil
		},
		GetFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, domain.ErrPersonaFileNotFound // Files don't exist yet
		},
	}

	mockLoader := &MockTemplateLoader{
		LoadFunc: func(fileType domain.PersonaFileType) (string, error) {
			return "Template for " + fileType.String(), nil
		},
	}

	var output bytes.Buffer
	cmd := cli.NewPersonaCommand(mockRepo, mockLoader, &output)

	ctx := context.Background()
	err := cmd.Init(ctx, "user1")

	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify all three files were created
	if len(savedFiles) != 3 {
		t.Errorf("Expected 3 files to be saved, got %d", len(savedFiles))
	}

	// Verify content for each file
	expectedFiles := []domain.PersonaFileType{
		domain.PersonaFileSOUL,
		domain.PersonaFileUSER,
		domain.PersonaFileRULES,
	}

	for _, ft := range expectedFiles {
		content, ok := savedFiles[ft]
		if !ok {
			t.Errorf("File %s was not saved", ft.String())
			continue
		}
		expected := "Template for " + ft.String()
		if content != expected {
			t.Errorf("File %s has wrong content. Expected %q, got %q", ft.String(), expected, content)
		}
	}

	// Verify output message
	outputStr := output.String()
	if !strings.Contains(outputStr, "Persona files initialized") {
		t.Errorf("Expected success message in output, got: %s", outputStr)
	}
}

func TestPersonaCommand_Init_FilesAlreadyExist(t *testing.T) {
	mockRepo := &MockPersonaFileRepository{
		GetFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			// Simulate existing SOUL file
			if fileType == domain.PersonaFileSOUL {
				return &domain.PersonaFile{
					Type:    fileType,
					Content: "existing content",
				}, nil
			}
			return nil, domain.ErrPersonaFileNotFound
		},
	}

	mockLoader := &MockTemplateLoader{}
	var output bytes.Buffer
	cmd := cli.NewPersonaCommand(mockRepo, mockLoader, &output)

	ctx := context.Background()
	err := cmd.Init(ctx, "user1")

	// Should fail because SOUL.md already exists
	if err == nil {
		t.Fatal("Expected error when files already exist, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

func TestPersonaCommand_Init_TemplateLoadFailure(t *testing.T) {
	mockRepo := &MockPersonaFileRepository{
		GetFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, domain.ErrPersonaFileNotFound
		},
	}

	mockLoader := &MockTemplateLoader{
		LoadFunc: func(fileType domain.PersonaFileType) (string, error) {
			return "", errors.New("template not found")
		},
	}

	var output bytes.Buffer
	cmd := cli.NewPersonaCommand(mockRepo, mockLoader, &output)

	ctx := context.Background()
	err := cmd.Init(ctx, "user1")

	if err == nil {
		t.Fatal("Expected error when template load fails, got nil")
	}
}

func TestPersonaCommand_Init_SaveFailure(t *testing.T) {
	mockRepo := &MockPersonaFileRepository{
		GetFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, domain.ErrPersonaFileNotFound
		},
		SaveFunc: func(ctx context.Context, file *domain.PersonaFile) error {
			return errors.New("disk full")
		},
	}

	mockLoader := &MockTemplateLoader{}
	var output bytes.Buffer
	cmd := cli.NewPersonaCommand(mockRepo, mockLoader, &output)

	ctx := context.Background()
	err := cmd.Init(ctx, "user1")

	if err == nil {
		t.Fatal("Expected error when save fails, got nil")
	}

	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("Expected 'disk full' error, got: %v", err)
	}
}

func TestPersonaCommand_Init_EmptyUserID(t *testing.T) {
	mockRepo := &MockPersonaFileRepository{}
	mockLoader := &MockTemplateLoader{}
	var output bytes.Buffer
	cmd := cli.NewPersonaCommand(mockRepo, mockLoader, &output)

	ctx := context.Background()
	err := cmd.Init(ctx, "")

	if err == nil {
		t.Fatal("Expected error for empty userID, got nil")
	}
}
