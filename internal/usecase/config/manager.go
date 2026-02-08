package config

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// ConfigLoader defines the interface for loading and validating configuration.
type ConfigLoader interface {
	// Load loads the configuration from files
	Load() (*config.NuimanBotConfig, error)

	// Validate validates the loaded configuration
	Validate(*config.NuimanBotConfig) error
}

// ConfigManager manages hot-reload of application configuration.
// It provides thread-safe access to the current configuration and supports
// atomic updates with validation and listener notification.
//
// Thread Safety:
//   - Uses sync.RWMutex to allow many concurrent readers with single writer
//   - GetConfig() acquires read lock (multiple readers allowed)
//   - Reload() acquires write lock (exclusive access)
//
// Reload Flow:
//  1. Load new configuration from files
//  2. Validate new configuration
//  3. Acquire write lock
//  4. Swap configuration atomically
//  5. Release lock
//  6. Notify listeners (continues even if listener fails)
//  7. Audit log the operation
//
// Rollback:
//   - If loading fails: no changes made, old config remains
//   - If validation fails: no changes made, old config remains
//   - If listener fails: changes still applied, error logged
type ConfigManager struct {
	// currentConfig holds the active configuration
	currentConfig *config.NuimanBotConfig

	// mu protects currentConfig for thread-safe access
	mu sync.RWMutex

	// loader is used to load and validate configuration
	loader ConfigLoader

	// securityService is used for audit logging (optional)
	securityService domain.SecurityService

	// listeners are notified when config is reloaded
	listeners []domain.ConfigReloadListener

	// listenersMu protects the listeners slice
	listenersMu sync.RWMutex
}

// NewConfigManager creates a new ConfigManager with the initial configuration.
//
// Parameters:
//   - initialConfig: The initial configuration to use
//   - loader: The config loader for reloading configuration
//   - securityService: Optional security service for audit logging (can be nil)
//
// Returns:
//   - *ConfigManager: The initialized config manager
func NewConfigManager(
	initialConfig *config.NuimanBotConfig,
	loader ConfigLoader,
	securityService domain.SecurityService,
) *ConfigManager {
	return &ConfigManager{
		currentConfig:   initialConfig,
		loader:          loader,
		securityService: securityService,
		listeners:       make([]domain.ConfigReloadListener, 0),
	}
}

// GetConfig returns the current configuration in a thread-safe manner.
// Uses read lock to allow multiple concurrent readers.
//
// Returns:
//   - *config.NuimanBotConfig: The current configuration (read-only access)
func (cm *ConfigManager) GetConfig() *config.NuimanBotConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.currentConfig
}

// RegisterListener registers a listener to be notified on config reload.
// Listeners are called in the order they are registered.
//
// Parameters:
//   - listener: The listener to register
//
// Thread Safety:
//   - Safe to call concurrently
//   - Safe to call during a reload (will be notified on next reload)
func (cm *ConfigManager) RegisterListener(listener domain.ConfigReloadListener) {
	cm.listenersMu.Lock()
	defer cm.listenersMu.Unlock()
	cm.listeners = append(cm.listeners, listener)
}

// Reload reloads the configuration from files with atomic swap and listener notification.
//
// Flow:
//  1. Check context for cancellation
//  2. Load new configuration from files
//  3. Validate new configuration
//  4. Acquire write lock
//  5. Swap configuration atomically
//  6. Release write lock
//  7. Notify all registered listeners (continues even if listener fails)
//  8. Audit log the operation (success or failure)
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//
// Returns:
//   - error: Error if reload fails (load error, validation error, context cancelled)
//     Note: Listener errors are logged but do not fail the reload
//
// Rollback:
//   - On load failure: old config remains unchanged
//   - On validation failure: old config remains unchanged
//   - On listener failure: new config is kept, error is logged
//
// Thread Safety:
//   - Safe to call concurrently (uses write lock for config swap)
//   - Readers can continue reading old config during load/validation
//   - Write lock held only during config swap (minimal blocking time)
func (cm *ConfigManager) Reload(ctx context.Context) error {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		errMsg := fmt.Sprintf("context cancelled: %v", err)
		cm.auditReload(ctx, "failure", errMsg)
		return fmt.Errorf("reload cancelled: %w", err)
	}

	// Load new configuration
	newConfig, err := cm.loadConfig()
	if err != nil {
		cm.auditReload(ctx, "failure", fmt.Sprintf("load error: %v", err))
		return err
	}

	// Check context again after potentially long load operation
	if err := ctx.Err(); err != nil {
		errMsg := fmt.Sprintf("context cancelled after load: %v", err)
		cm.auditReload(ctx, "failure", errMsg)
		return fmt.Errorf("reload cancelled after load: %w", err)
	}

	// Validate new configuration
	if err := cm.validateConfig(newConfig); err != nil {
		cm.auditReload(ctx, "failure", fmt.Sprintf("validation error: %v", err))
		return err
	}

	// Swap configuration atomically
	cm.swapConfig(newConfig)

	// Notify listeners (after releasing write lock)
	cm.notifyListeners(ctx, newConfig)

	// Audit log success
	cm.auditReload(ctx, "success", "configuration reloaded successfully")

	return nil
}

// loadConfig loads the configuration using the configured loader.
func (cm *ConfigManager) loadConfig() (*config.NuimanBotConfig, error) {
	newConfig, err := cm.loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return newConfig, nil
}

// validateConfig validates the configuration using the configured loader.
func (cm *ConfigManager) validateConfig(cfg *config.NuimanBotConfig) error {
	if err := cm.loader.Validate(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	return nil
}

// swapConfig atomically replaces the current configuration with a new one.
func (cm *ConfigManager) swapConfig(newConfig *config.NuimanBotConfig) {
	cm.mu.Lock()
	cm.currentConfig = newConfig
	cm.mu.Unlock()
}

// notifyListeners notifies all registered listeners of config reload.
// Listener errors are logged but do not fail the reload operation.
//
// Parameters:
//   - ctx: Context for cancellation
//   - newConfig: The newly loaded configuration
//
// Thread Safety:
//   - Acquires read lock on listeners slice
//   - Listeners are called sequentially (not concurrently)
func (cm *ConfigManager) notifyListeners(ctx context.Context, newConfig *config.NuimanBotConfig) {
	cm.listenersMu.RLock()
	listeners := make([]domain.ConfigReloadListener, len(cm.listeners))
	copy(listeners, cm.listeners)
	cm.listenersMu.RUnlock()

	for i, listener := range listeners {
		if err := listener.OnConfigReload(ctx, newConfig); err != nil {
			// Log error but continue notifying other listeners
			log.Printf("ERROR: Listener %d failed to handle config reload: %v", i, err)
		}
	}
}

// auditReload logs the reload operation to the audit log.
//
// Parameters:
//   - ctx: Context (may contain user/request info)
//   - outcome: "success" or "failure"
//   - details: Human-readable details about the operation
func (cm *ConfigManager) auditReload(ctx context.Context, outcome, details string) {
	if cm.securityService == nil {
		// No security service configured, skip audit logging
		return
	}

	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "system", // Config reload is typically a system operation
		Action:    "config_reload",
		Resource:  "configuration",
		Outcome:   outcome,
		Details: map[string]any{
			"details": details,
		},
	}

	if err := cm.securityService.Audit(ctx, event); err != nil {
		// Log error but don't fail the operation
		log.Printf("ERROR: Failed to write audit log: %v", err)
	}
}
