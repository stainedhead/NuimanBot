package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// MockSkillExecutor is a mock implementation of SkillExecutor for testing
type MockSkillExecutor struct {
	executeErr     error
	listErr        error
	describeErr    error
	executeCalled  bool
	listCalled     bool
	describeCalled bool
	lastSkillName  string
	lastArgs       []string
}

func (m *MockSkillExecutor) Execute(ctx context.Context, skillName string, args []string) (*domain.RenderedSkill, error) {
	m.executeCalled = true
	m.lastSkillName = skillName
	m.lastArgs = args
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return &domain.RenderedSkill{
		SkillName:    skillName,
		Prompt:       "Test prompt",
		AllowedTools: []string{"tool1", "tool2"},
	}, nil
}

func (m *MockSkillExecutor) List(ctx context.Context) error {
	m.listCalled = true
	if m.listErr != nil {
		return m.listErr
	}
	return nil
}

func (m *MockSkillExecutor) Describe(ctx context.Context, skillName string) error {
	m.describeCalled = true
	m.lastSkillName = skillName
	if m.describeErr != nil {
		return m.describeErr
	}
	return nil
}

func TestSkillHandler_Execute(t *testing.T) {
	tests := []struct {
		name        string
		skillName   string
		args        []string
		executeErr  error
		expectError bool
	}{
		{
			name:        "Valid skill execution",
			skillName:   "test-skill",
			args:        []string{"arg1", "arg2"},
			executeErr:  nil,
			expectError: false,
		},
		{
			name:        "Skill not found",
			skillName:   "nonexistent",
			args:        []string{},
			executeErr:  domain.ErrSkillNotFound{SkillName: "nonexistent"},
			expectError: true,
		},
		{
			name:        "Skill with no args",
			skillName:   "simple-skill",
			args:        []string{},
			executeErr:  nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := &MockSkillExecutor{executeErr: tt.executeErr}
			output := &bytes.Buffer{}
			handler := cli.NewSkillHandler(mockExecutor, output)

			err := handler.Execute(context.Background(), tt.skillName, tt.args)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}

			if !mockExecutor.executeCalled {
				t.Error("Execute was not called on mock executor")
			}
			if mockExecutor.lastSkillName != tt.skillName {
				t.Errorf("Expected skill name '%s', got '%s'", tt.skillName, mockExecutor.lastSkillName)
			}

			// Check output for successful execution
			if !tt.expectError {
				outputStr := output.String()
				if !strings.Contains(outputStr, "[Skill activated:") {
					t.Errorf("Output should contain skill activation message, got: %s", outputStr)
				}
				if !strings.Contains(outputStr, "Test prompt") {
					t.Errorf("Output should contain prompt, got: %s", outputStr)
				}
			}
		})
	}
}

func TestSkillHandler_List(t *testing.T) {
	mockExecutor := &MockSkillExecutor{}
	output := &bytes.Buffer{}
	handler := cli.NewSkillHandler(mockExecutor, output)

	err := handler.List(context.Background())

	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if !mockExecutor.listCalled {
		t.Error("List was not called on mock executor")
	}
}

func TestSkillHandler_Describe(t *testing.T) {
	mockExecutor := &MockSkillExecutor{}
	output := &bytes.Buffer{}
	handler := cli.NewSkillHandler(mockExecutor, output)

	err := handler.Describe(context.Background(), "test-skill")

	if err != nil {
		t.Fatalf("Describe returned error: %v", err)
	}
	if !mockExecutor.describeCalled {
		t.Error("Describe was not called on mock executor")
	}
	if mockExecutor.lastSkillName != "test-skill" {
		t.Errorf("Expected skill name 'test-skill', got '%s'", mockExecutor.lastSkillName)
	}
}

func TestGateway_SkillCommandHandling(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedSkillName string
		expectedArgs      []string
		expectSkillCall   bool
	}{
		{
			name:              "Help command",
			input:             "/help\nexit\n",
			expectedSkillName: "",
			expectedArgs:      nil,
			expectSkillCall:   true,
		},
		{
			name:              "Skill with no args",
			input:             "/test-skill\nexit\n",
			expectedSkillName: "test-skill",
			expectedArgs:      []string{},
			expectSkillCall:   true,
		},
		{
			name:              "Skill with args",
			input:             "/code-review file.go main.go\nexit\n",
			expectedSkillName: "code-review",
			expectedArgs:      []string{"file.go", "main.go"},
			expectSkillCall:   true,
		},
		{
			name:              "Describe command",
			input:             "/describe test-skill\nexit\n",
			expectedSkillName: "",
			expectedArgs:      nil,
			expectSkillCall:   true,
		},
		{
			name:              "Regular message (not a command)",
			input:             "hello world\nexit\n",
			expectedSkillName: "",
			expectedArgs:      nil,
			expectSkillCall:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.CLIConfig{}
			output := &bytes.Buffer{}
			g, readerPipe, writerPipe := newTestGateway(cfg, tt.input, output)

			defer func() {
				readerPipe.Close()
				writerPipe.Close()
			}()

			mockExecutor := &MockSkillExecutor{}
			skillHandler := cli.NewSkillHandler(mockExecutor, output)
			g.SetSkillHandler(skillHandler)

			// Set message handler to catch non-command messages
			messageReceived := false
			g.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
				messageReceived = true
				return nil
			})

			ctx := context.Background()
			err := g.Start(ctx)
			if err != nil {
				t.Fatalf("Start returned error: %v", err)
			}

			if tt.expectSkillCall {
				if !mockExecutor.executeCalled && !mockExecutor.listCalled && !mockExecutor.describeCalled {
					t.Error("Expected skill handler to be called but it wasn't")
				}
				if tt.expectedSkillName != "" && mockExecutor.lastSkillName != tt.expectedSkillName {
					t.Errorf("Expected skill name '%s', got '%s'", tt.expectedSkillName, mockExecutor.lastSkillName)
				}
			} else {
				if mockExecutor.executeCalled {
					t.Error("Skill handler should not be called for non-command messages")
				}
				if !messageReceived {
					t.Error("Message handler should be called for regular messages")
				}
			}
		})
	}
}

func TestGateway_SkillError(t *testing.T) {
	cfg := &config.CLIConfig{}
	input := "/nonexistent-skill\nexit\n"
	output := &bytes.Buffer{}
	g, readerPipe, writerPipe := newTestGateway(cfg, input, output)

	defer func() {
		readerPipe.Close()
		writerPipe.Close()
	}()

	mockExecutor := &MockSkillExecutor{
		executeErr: errors.New("skill not found: nonexistent-skill"),
	}
	skillHandler := cli.NewSkillHandler(mockExecutor, output)
	g.SetSkillHandler(skillHandler)

	ctx := context.Background()
	err := g.Start(ctx)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Error:") {
		t.Errorf("Expected error message in output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "skill not found") {
		t.Errorf("Expected 'skill not found' in error output, got: %s", outputStr)
	}
}

func TestGateway_NoSkillHandler(t *testing.T) {
	// Test that commands fall through to message handler when no skill handler is set
	cfg := &config.CLIConfig{}
	input := "/some-command\nexit\n"
	output := &bytes.Buffer{}
	g, readerPipe, writerPipe := newTestGateway(cfg, input, output)

	defer func() {
		readerPipe.Close()
		writerPipe.Close()
	}()

	// Don't set a skill handler
	messageReceived := false
	var receivedText string
	g.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		messageReceived = true
		receivedText = msg.Text
		return nil
	})

	ctx := context.Background()
	err := g.Start(ctx)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// Without a skill handler, the command should be treated as a regular message
	if !messageReceived {
		t.Error("Message handler should be called when no skill handler is set")
	}
	if receivedText != "/some-command" {
		t.Errorf("Expected message text '/some-command', got '%s'", receivedText)
	}
}

func TestGateway_DescribeCommand(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Describe with skill name",
			input:       "/describe test-skill\nexit\n",
			expectError: false,
		},
		{
			name:        "Describe without skill name",
			input:       "/describe\nexit\n",
			expectError: true,
			errorMsg:    "usage: /describe <skill-name>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.CLIConfig{}
			output := &bytes.Buffer{}
			g, readerPipe, writerPipe := newTestGateway(cfg, tt.input, output)

			defer func() {
				readerPipe.Close()
				writerPipe.Close()
			}()

			mockExecutor := &MockSkillExecutor{}
			skillHandler := cli.NewSkillHandler(mockExecutor, output)
			g.SetSkillHandler(skillHandler)

			ctx := context.Background()
			err := g.Start(ctx)
			if err != nil {
				t.Fatalf("Start returned error: %v", err)
			}

			outputStr := output.String()
			if tt.expectError {
				if !strings.Contains(outputStr, "Error:") {
					t.Errorf("Expected error message in output, got: %s", outputStr)
				}
				if tt.errorMsg != "" && !strings.Contains(outputStr, tt.errorMsg) {
					t.Errorf("Expected error message '%s' in output, got: %s", tt.errorMsg, outputStr)
				}
			} else {
				if !mockExecutor.describeCalled {
					t.Error("Describe should be called on skill executor")
				}
			}
		})
	}
}
