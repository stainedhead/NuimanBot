package testutil

import (
	"context"
	"nuimanbot/internal/usecase/tool/executor"
)

// MockExecutor is a mock implementation of ExecutorService for testing
type MockExecutor struct {
	ExecuteFunc           func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error)
	ExecuteBackgroundFunc func(ctx context.Context, req executor.ExecutionRequest) (*executor.BackgroundSession, error)
	GetSessionStatusFunc  func(ctx context.Context, sessionID string) (*executor.SessionStatus, error)
	GetSessionOutputFunc  func(ctx context.Context, sessionID string) (string, error)
	CancelSessionFunc     func(ctx context.Context, sessionID string) error
}

// NewMockExecutor creates a new MockExecutor with default no-op implementations
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		ExecuteFunc: func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
			return &executor.ExecutionResult{
				Stdout:   "",
				Stderr:   "",
				ExitCode: 0,
			}, nil
		},
		ExecuteBackgroundFunc: func(ctx context.Context, req executor.ExecutionRequest) (*executor.BackgroundSession, error) {
			return nil, nil
		},
		GetSessionStatusFunc: func(ctx context.Context, sessionID string) (*executor.SessionStatus, error) {
			return nil, nil
		},
		GetSessionOutputFunc: func(ctx context.Context, sessionID string) (string, error) {
			return "", nil
		},
		CancelSessionFunc: func(ctx context.Context, sessionID string) error {
			return nil
		},
	}
}

// Execute delegates to ExecuteFunc
func (m *MockExecutor) Execute(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, req)
	}
	return &executor.ExecutionResult{}, nil
}

// ExecuteBackground delegates to ExecuteBackgroundFunc
func (m *MockExecutor) ExecuteBackground(ctx context.Context, req executor.ExecutionRequest) (*executor.BackgroundSession, error) {
	if m.ExecuteBackgroundFunc != nil {
		return m.ExecuteBackgroundFunc(ctx, req)
	}
	return nil, nil
}

// GetSessionStatus delegates to GetSessionStatusFunc
func (m *MockExecutor) GetSessionStatus(ctx context.Context, sessionID string) (*executor.SessionStatus, error) {
	if m.GetSessionStatusFunc != nil {
		return m.GetSessionStatusFunc(ctx, sessionID)
	}
	return nil, nil
}

// GetSessionOutput delegates to GetSessionOutputFunc
func (m *MockExecutor) GetSessionOutput(ctx context.Context, sessionID string) (string, error) {
	if m.GetSessionOutputFunc != nil {
		return m.GetSessionOutputFunc(ctx, sessionID)
	}
	return "", nil
}

// CancelSession delegates to CancelSessionFunc
func (m *MockExecutor) CancelSession(ctx context.Context, sessionID string) error {
	if m.CancelSessionFunc != nil {
		return m.CancelSessionFunc(ctx, sessionID)
	}
	return nil
}
