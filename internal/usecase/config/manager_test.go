package config

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// mockConfigLoader is a mock implementation of config loading
type mockConfigLoader struct {
	loadFunc     func() (*config.NuimanBotConfig, error)
	validateFunc func(*config.NuimanBotConfig) error
}

func (m *mockConfigLoader) Load() (*config.NuimanBotConfig, error) {
	if m.loadFunc != nil {
		return m.loadFunc()
	}
	return &config.NuimanBotConfig{}, nil
}

func (m *mockConfigLoader) Validate(cfg *config.NuimanBotConfig) error {
	if m.validateFunc != nil {
		return m.validateFunc(cfg)
	}
	return nil
}

// mockSecurityService is a mock implementation of domain.SecurityService
type mockSecurityService struct {
	auditFunc func(context.Context, *domain.AuditEvent) error
}

func (m *mockSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (m *mockSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (m *mockSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	return input, nil
}

func (m *mockSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if m.auditFunc != nil {
		return m.auditFunc(ctx, event)
	}
	return nil
}

func (m *mockSecurityService) GenerateAPIKey(ctx context.Context) (string, error) {
	return "mock-api-key-12345678", nil
}

// mockConfigReloadListener is a mock implementation of domain.ConfigReloadListener
type mockConfigReloadListener struct {
	onReloadFunc func(context.Context, *config.NuimanBotConfig) error
	callCount    int
	mu           sync.Mutex
}

func (m *mockConfigReloadListener) OnConfigReload(ctx context.Context, newConfig interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.onReloadFunc != nil {
		cfg, ok := newConfig.(*config.NuimanBotConfig)
		if !ok {
			return fmt.Errorf("expected *config.NuimanBotConfig, got %T", newConfig)
		}
		return m.onReloadFunc(ctx, cfg)
	}
	return nil
}

func (m *mockConfigReloadListener) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// TestConfigManager_GetConfig tests concurrent safe reads
func TestConfigManager_GetConfig(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return initialCfg, nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, nil)

	// Test single read
	cfg := mgr.GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig returned nil")
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected LogLevel=info, got %s", cfg.Server.LogLevel)
	}

	// Test concurrent reads
	const numReaders = 100
	var wg sync.WaitGroup
	wg.Add(numReaders)

	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			cfg := mgr.GetConfig()
			if cfg == nil {
				t.Error("GetConfig returned nil in concurrent access")
			}
		}()
	}

	wg.Wait()
}

// TestConfigManager_Reload_Success tests successful configuration reload
func TestConfigManager_Reload_Success(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	newCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "debug",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return newCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return nil
		},
	}

	auditCalled := false
	securitySvc := &mockSecurityService{
		auditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditCalled = true
			if event.Action != "config_reload" {
				t.Errorf("Expected Action=config_reload, got %s", event.Action)
			}
			if event.Outcome != "success" {
				t.Errorf("Expected Outcome=success, got %s", event.Outcome)
			}
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, securitySvc)

	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify config was updated
	cfg := mgr.GetConfig()
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected LogLevel=debug after reload, got %s", cfg.Server.LogLevel)
	}

	// Verify audit log was called
	if !auditCalled {
		t.Error("Audit log was not called")
	}
}

// TestConfigManager_Reload_ValidationFailure tests rollback on validation failure
func TestConfigManager_Reload_ValidationFailure(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	newCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "invalid",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return newCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return errors.New("invalid log level")
		},
	}

	auditCalled := false
	securitySvc := &mockSecurityService{
		auditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditCalled = true
			if event.Outcome != "failure" {
				t.Errorf("Expected Outcome=failure, got %s", event.Outcome)
			}
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, securitySvc)

	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err == nil {
		t.Fatal("Expected Reload to fail, but it succeeded")
	}

	// Verify config was NOT updated (rollback)
	cfg := mgr.GetConfig()
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected LogLevel=info after failed reload, got %s", cfg.Server.LogLevel)
	}

	// Verify audit log was called with failure
	if !auditCalled {
		t.Error("Audit log was not called")
	}
}

// TestConfigManager_Reload_LoadFailure tests handling of load errors
func TestConfigManager_Reload_LoadFailure(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return nil, errors.New("file not found")
		},
	}

	auditCalled := false
	securitySvc := &mockSecurityService{
		auditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditCalled = true
			if event.Outcome != "failure" {
				t.Errorf("Expected Outcome=failure, got %s", event.Outcome)
			}
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, securitySvc)

	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err == nil {
		t.Fatal("Expected Reload to fail, but it succeeded")
	}

	// Verify config was NOT updated
	cfg := mgr.GetConfig()
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected LogLevel=info after failed reload, got %s", cfg.Server.LogLevel)
	}

	// Verify audit log was called with failure
	if !auditCalled {
		t.Error("Audit log was not called")
	}
}

// TestConfigManager_RegisterListener tests listener registration
func TestConfigManager_RegisterListener(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	newCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "debug",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return newCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, nil)

	// Register listener
	listener := &mockConfigReloadListener{}
	mgr.RegisterListener(listener)

	// Trigger reload
	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify listener was notified
	if listener.getCallCount() != 1 {
		t.Errorf("Expected listener to be called once, got %d", listener.getCallCount())
	}
}

// TestConfigManager_MultipleListeners tests multiple listener notification
func TestConfigManager_MultipleListeners(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	newCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "debug",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return newCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, nil)

	// Register multiple listeners
	listener1 := &mockConfigReloadListener{}
	listener2 := &mockConfigReloadListener{}
	listener3 := &mockConfigReloadListener{}

	mgr.RegisterListener(listener1)
	mgr.RegisterListener(listener2)
	mgr.RegisterListener(listener3)

	// Trigger reload
	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify all listeners were notified
	if listener1.getCallCount() != 1 {
		t.Errorf("Expected listener1 to be called once, got %d", listener1.getCallCount())
	}
	if listener2.getCallCount() != 1 {
		t.Errorf("Expected listener2 to be called once, got %d", listener2.getCallCount())
	}
	if listener3.getCallCount() != 1 {
		t.Errorf("Expected listener3 to be called once, got %d", listener3.getCallCount())
	}
}

// TestConfigManager_ListenerError tests that listener errors don't fail reload
func TestConfigManager_ListenerError(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	newCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "debug",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return newCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, nil)

	// Register listener that returns error
	failingListener := &mockConfigReloadListener{
		onReloadFunc: func(ctx context.Context, cfg *config.NuimanBotConfig) error {
			return errors.New("listener failed to reinitialize")
		},
	}

	successListener := &mockConfigReloadListener{}

	mgr.RegisterListener(failingListener)
	mgr.RegisterListener(successListener)

	// Trigger reload - should succeed despite listener error
	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err != nil {
		t.Fatalf("Reload should not fail due to listener error: %v", err)
	}

	// Verify both listeners were called
	if failingListener.getCallCount() != 1 {
		t.Errorf("Expected failingListener to be called once, got %d", failingListener.getCallCount())
	}
	if successListener.getCallCount() != 1 {
		t.Errorf("Expected successListener to be called once, got %d", successListener.getCallCount())
	}

	// Verify config was updated despite listener error
	cfg := mgr.GetConfig()
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected LogLevel=debug after reload, got %s", cfg.Server.LogLevel)
	}
}

// TestConfigManager_ConcurrentReloadAndRead tests thread safety
func TestConfigManager_ConcurrentReloadAndRead(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	reloadCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "debug",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			// Simulate slow load
			time.Sleep(10 * time.Millisecond)
			return reloadCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, nil)

	// Start multiple readers and reloaders concurrently
	const numReaders = 50
	const numReloaders = 5
	var wg sync.WaitGroup

	// Readers
	wg.Add(numReaders)
	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cfg := mgr.GetConfig()
				if cfg == nil {
					t.Error("GetConfig returned nil")
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Reloaders
	wg.Add(numReloaders)
	for i := 0; i < numReloaders; i++ {
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_ = mgr.Reload(ctx)
		}()
	}

	wg.Wait()

	// Verify final config is valid
	cfg := mgr.GetConfig()
	if cfg == nil {
		t.Fatal("Final config is nil")
	}
}

// TestConfigManager_NilSecurityService tests operation without security service
func TestConfigManager_NilSecurityService(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	newCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "debug",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			return newCfg, nil
		},
		validateFunc: func(cfg *config.NuimanBotConfig) error {
			return nil
		},
	}

	// Create manager with nil security service
	mgr := NewConfigManager(initialCfg, loader, nil)

	ctx := context.Background()
	err := mgr.Reload(ctx)
	if err != nil {
		t.Fatalf("Reload should succeed with nil security service: %v", err)
	}

	// Verify config was updated
	cfg := mgr.GetConfig()
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected LogLevel=debug after reload, got %s", cfg.Server.LogLevel)
	}
}

// TestConfigManager_ContextCancellation tests context cancellation handling
func TestConfigManager_ContextCancellation(t *testing.T) {
	initialCfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			LogLevel: "info",
		},
	}

	loader := &mockConfigLoader{
		loadFunc: func() (*config.NuimanBotConfig, error) {
			// Simulate slow load
			time.Sleep(100 * time.Millisecond)
			return &config.NuimanBotConfig{
				Server: config.ServerConfig{
					LogLevel: "debug",
				},
			}, nil
		},
	}

	mgr := NewConfigManager(initialCfg, loader, nil)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mgr.Reload(ctx)
	if err == nil {
		t.Fatal("Expected Reload to fail with cancelled context")
	}

	// Verify config was NOT updated
	cfg := mgr.GetConfig()
	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected LogLevel=info after cancelled reload, got %s", cfg.Server.LogLevel)
	}
}
