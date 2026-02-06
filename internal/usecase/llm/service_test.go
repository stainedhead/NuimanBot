package llm

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// mockProviderClient is a mock implementation of ProviderClient for testing
type mockProviderClient struct {
	completeFunc        func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error)
	streamFunc          func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error)
	listModelsFunc      func(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error)
	completeCallCount   int
	streamCallCount     int
	listModelsCallCount int
}

func (m *mockProviderClient) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	m.completeCallCount++
	if m.completeFunc != nil {
		return m.completeFunc(ctx, provider, req)
	}
	return &domain.LLMResponse{Content: "mock response"}, nil
}

func (m *mockProviderClient) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	m.streamCallCount++
	if m.streamFunc != nil {
		return m.streamFunc(ctx, provider, req)
	}
	ch := make(chan domain.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (m *mockProviderClient) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	m.listModelsCallCount++
	if m.listModelsFunc != nil {
		return m.listModelsFunc(ctx, provider)
	}
	return []domain.ModelInfo{{ID: "mock-model", Name: "Mock Model"}}, nil
}

// TestNewService tests service creation
func TestNewService(t *testing.T) {
	cfg := &config.LLMConfig{}
	svc := NewService(cfg)

	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
	if svc.cfg != cfg {
		t.Error("NewService() did not set config correctly")
	}
}

// TestRegisterProviderClient tests provider registration
func TestRegisterProviderClient(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	mockClient := &mockProviderClient{}

	// Register Anthropic client
	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	// Verify registration
	client, err := svc.GetClientForProvider(domain.LLMProviderAnthropic)
	if err != nil {
		t.Errorf("GetClientForProvider() error = %v, want nil", err)
	}
	if client != mockClient {
		t.Error("GetClientForProvider() returned different client than registered")
	}
}

// TestGetClientForProvider_NotRegistered tests error when provider not registered
func TestGetClientForProvider_NotRegistered(t *testing.T) {
	svc := NewService(&config.LLMConfig{})

	// Try to get unregistered provider
	_, err := svc.GetClientForProvider(domain.LLMProviderOpenAI)
	if err == nil {
		t.Error("GetClientForProvider() error = nil, want error for unregistered provider")
	}

	expectedErrMsg := "LLM client for provider openai not registered"
	if err.Error() != expectedErrMsg {
		t.Errorf("GetClientForProvider() error = %v, want %v", err.Error(), expectedErrMsg)
	}
}

// TestRegisterMultipleProviders tests registering multiple providers
func TestRegisterMultipleProviders(t *testing.T) {
	svc := NewService(&config.LLMConfig{})

	anthropicClient := &mockProviderClient{}
	openaiClient := &mockProviderClient{}
	ollamaClient := &mockProviderClient{}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, anthropicClient)
	svc.RegisterProviderClient(domain.LLMProviderOpenAI, openaiClient)
	svc.RegisterProviderClient(domain.LLMProviderOllama, ollamaClient)

	// Verify all registrations
	tests := []struct {
		provider domain.LLMProvider
		expected ProviderClient
	}{
		{domain.LLMProviderAnthropic, anthropicClient},
		{domain.LLMProviderOpenAI, openaiClient},
		{domain.LLMProviderOllama, ollamaClient},
	}

	for _, tt := range tests {
		client, err := svc.GetClientForProvider(tt.provider)
		if err != nil {
			t.Errorf("GetClientForProvider(%s) error = %v, want nil", tt.provider, err)
		}
		if client != tt.expected {
			t.Errorf("GetClientForProvider(%s) returned wrong client", tt.provider)
		}
	}
}

// TestComplete_Success tests successful completion routing
func TestComplete_Success(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	mockClient := &mockProviderClient{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{
				Content:      "Hello from " + string(provider),
				FinishReason: "end_turn",
			}, nil
		},
	}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := svc.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Errorf("Complete() error = %v, want nil", err)
	}
	if resp == nil {
		t.Fatal("Complete() returned nil response")
	}
	if resp.Content != "Hello from anthropic" {
		t.Errorf("Complete() content = %v, want 'Hello from anthropic'", resp.Content)
	}
	if mockClient.completeCallCount != 1 {
		t.Errorf("Complete() call count = %d, want 1", mockClient.completeCallCount)
	}
}

// TestComplete_ProviderNotRegistered tests completion with unregistered provider
func TestComplete_ProviderNotRegistered(t *testing.T) {
	svc := NewService(&config.LLMConfig{})

	req := &domain.LLMRequest{
		Model:     "gpt-4",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := svc.Complete(context.Background(), domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Complete() error = nil, want error for unregistered provider")
	}
}

// TestComplete_ClientError tests completion when client returns error
func TestComplete_ClientError(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	expectedErr := errors.New("API rate limit exceeded")
	mockClient := &mockProviderClient{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, expectedErr
		},
	}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := svc.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Error("Complete() error = nil, want error from client")
	}
	if err != expectedErr {
		t.Errorf("Complete() error = %v, want %v", err, expectedErr)
	}
}

// TestStream_Success tests successful stream routing
func TestStream_Success(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	mockClient := &mockProviderClient{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			ch := make(chan domain.StreamChunk, 2)
			ch <- domain.StreamChunk{Delta: "Hello"}
			ch <- domain.StreamChunk{Done: true}
			close(ch)
			return ch, nil
		},
	}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := svc.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Errorf("Stream() error = %v, want nil", err)
	}
	if ch == nil {
		t.Fatal("Stream() returned nil channel")
	}

	// Read from channel
	chunks := []domain.StreamChunk{}
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Errorf("Stream() received %d chunks, want 2", len(chunks))
	}
	if mockClient.streamCallCount != 1 {
		t.Errorf("Stream() call count = %d, want 1", mockClient.streamCallCount)
	}
}

// TestStream_ProviderNotRegistered tests stream with unregistered provider
func TestStream_ProviderNotRegistered(t *testing.T) {
	svc := NewService(&config.LLMConfig{})

	req := &domain.LLMRequest{
		Model:     "gpt-4",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := svc.Stream(context.Background(), domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Stream() error = nil, want error for unregistered provider")
	}
}

// TestStream_ClientError tests stream when client returns error
func TestStream_ClientError(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	expectedErr := errors.New("streaming not supported")
	mockClient := &mockProviderClient{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			return nil, expectedErr
		},
	}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := svc.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Error("Stream() error = nil, want error from client")
	}
	if err != expectedErr {
		t.Errorf("Stream() error = %v, want %v", err, expectedErr)
	}
}

// TestListModels_Success tests successful model listing
func TestListModels_Success(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	mockModels := []domain.ModelInfo{
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: "anthropic", ContextWindow: 200000},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic", ContextWindow: 200000},
	}
	mockClient := &mockProviderClient{
		listModelsFunc: func(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
			return mockModels, nil
		},
	}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	models, err := svc.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err != nil {
		t.Errorf("ListModels() error = %v, want nil", err)
	}
	if len(models) != 2 {
		t.Errorf("ListModels() returned %d models, want 2", len(models))
	}
	if mockClient.listModelsCallCount != 1 {
		t.Errorf("ListModels() call count = %d, want 1", mockClient.listModelsCallCount)
	}
}

// TestListModels_ProviderNotRegistered tests ListModels with unregistered provider
func TestListModels_ProviderNotRegistered(t *testing.T) {
	svc := NewService(&config.LLMConfig{})

	_, err := svc.ListModels(context.Background(), domain.LLMProviderOpenAI)
	if err == nil {
		t.Error("ListModels() error = nil, want error for unregistered provider")
	}
}

// TestListModels_ClientError tests ListModels when client returns error
func TestListModels_ClientError(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	expectedErr := errors.New("API unavailable")
	mockClient := &mockProviderClient{
		listModelsFunc: func(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
			return nil, expectedErr
		},
	}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	_, err := svc.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err == nil {
		t.Error("ListModels() error = nil, want error from client")
	}
	if err != expectedErr {
		t.Errorf("ListModels() error = %v, want %v", err, expectedErr)
	}
}

// TestConcurrentProviderAccess tests concurrent access to provider clients
func TestConcurrentProviderAccess(t *testing.T) {
	svc := NewService(&config.LLMConfig{})
	mockClient := &mockProviderClient{}

	svc.RegisterProviderClient(domain.LLMProviderAnthropic, mockClient)

	// Concurrent reads should not panic
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = svc.GetClientForProvider(domain.LLMProviderAnthropic)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
