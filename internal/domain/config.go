package domain

import (
	"context"
)

// ConfigReloadListener is notified when configuration is reloaded.
// Services that maintain state dependent on configuration should implement
// this interface and register with the ConfigManager to receive notifications
// when configuration changes.
//
// Example implementations:
//   - Gateway services that need to reconnect with new credentials
//   - LLM provider managers that need to update provider configurations
//   - Rate limiters that need to adjust limits based on new config
type ConfigReloadListener interface {
	// OnConfigReload is called after configuration has been successfully reloaded
	// and validated. Implementations should reinitialize any state that depends
	// on the configuration.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - newConfig: The newly loaded and validated configuration (as interface{} to avoid circular dependency)
	//     Cast to *config.NuimanBotConfig in implementations
	//
	// Returns:
	//   - error: If the listener fails to reinitialize. The error is logged but
	//     does not prevent other listeners from being notified or the reload
	//     from completing successfully.
	//
	// Thread Safety:
	//   - This method may be called concurrently if multiple reloads happen
	//     simultaneously. Implementations must be thread-safe.
	//
	// Best Practices:
	//   - Keep reinitializations fast (< 1 second if possible)
	//   - Respect the context for cancellation
	//   - Log errors but don't panic
	//   - Be idempotent (calling multiple times should be safe)
	OnConfigReload(ctx context.Context, newConfig interface{}) error
}
