// Package config provides hot-reload configuration management for NuimanBot.
//
// The ConfigManager provides thread-safe access to application configuration
// with support for atomic updates, validation, and listener notification.
//
// # Architecture
//
// The package follows Clean Architecture principles:
//
//   - Domain: ConfigReloadListener interface (internal/domain/config.go)
//   - Use Case: ConfigManager (internal/usecase/config/manager.go)
//   - Infrastructure: ConfigLoader implementations (to be provided by caller)
//
// # Key Features
//
//   - Thread-safe configuration access (sync.RWMutex)
//   - Atomic configuration updates
//   - Configuration validation before application
//   - Automatic rollback on validation failure
//   - Listener notification system
//   - Audit logging support
//   - Context-aware cancellation
//
// # Usage Example
//
// Basic usage with hot reload:
//
//	// 1. Load initial configuration
//	initialConfig, err := config.LoadConfig()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 2. Create config loader
//	loader := &RealConfigLoader{}
//
//	// 3. Create ConfigManager
//	mgr := config.NewConfigManager(initialConfig, loader, securityService)
//
//	// 4. Register services that need reload notifications
//	mgr.RegisterListener(gatewayService)
//	mgr.RegisterListener(llmProviderManager)
//
//	// 5. Get current config (thread-safe)
//	cfg := mgr.GetConfig()
//
//	// 6. Reload configuration (triggered by /refresh command)
//	ctx := context.Background()
//	if err := mgr.Reload(ctx); err != nil {
//	    log.Printf("Reload failed: %v", err)
//	}
//
// # Implementing ConfigReloadListener
//
// Services that depend on configuration should implement ConfigReloadListener:
//
//	type MyService struct {
//	    config *config.NuimanBotConfig
//	}
//
//	func (s *MyService) OnConfigReload(ctx context.Context, newConfig interface{}) error {
//	    cfg, ok := newConfig.(*config.NuimanBotConfig)
//	    if !ok {
//	        return fmt.Errorf("invalid config type")
//	    }
//
//	    // Reinitialize service state
//	    s.config = cfg
//	    // Reconnect to external services if needed
//	    // Update rate limits, timeouts, etc.
//
//	    return nil
//	}
//
// # Thread Safety
//
// ConfigManager is designed for high concurrency:
//
//   - GetConfig() uses read lock (multiple concurrent readers)
//   - Reload() uses write lock only during config swap (minimal blocking)
//   - Listeners are notified outside of write lock
//   - RegisterListener() is thread-safe
//
// # Error Handling
//
// Reload errors are handled differently based on the failure point:
//
//   - Load failure: No changes made, error returned
//   - Validation failure: No changes made, error returned
//   - Listener failure: Config is updated, error logged but not returned
//
// # Testing
//
// The package provides extensive test coverage (>90%):
//
//   - Unit tests for all methods
//   - Concurrent access tests
//   - Validation failure tests
//   - Listener error handling tests
//   - Context cancellation tests
//
// See manager_test.go for test examples.
package config
