package alerting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// ChannelType defines the alerting channel type.
type ChannelType string

const (
	ChannelTypeLog       ChannelType = "log"        // Log-based alerting
	ChannelTypeSlack     ChannelType = "slack"      // Slack webhooks
	ChannelTypePagerDuty ChannelType = "pagerduty"  // PagerDuty integration
	ChannelTypeEmail     ChannelType = "email"      // Email notifications
)

// Severity defines alert severity levels.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Config defines alerting configuration.
type Config struct {
	Enabled        bool
	ServiceName    string
	Channels       []ChannelConfig
	ThrottleWindow int // Seconds to throttle duplicate alerts
}

// ChannelConfig defines configuration for an alerting channel.
type ChannelConfig struct {
	Type    ChannelType
	Enabled bool
	Config  map[string]string // Channel-specific configuration
}

// Alert represents an alert to be sent.
type Alert struct {
	Severity Severity
	Title    string
	Message  string
	Tags     map[string]string
	Details  map[string]any
}

var (
	globalConfig  Config
	initialized   bool
	mu            sync.RWMutex
	throttleCache map[string]time.Time // Alert fingerprint -> last sent time
	throttleMu    sync.RWMutex
)

// Initialize sets up the alerting system.
func Initialize(config Config) error {
	mu.Lock()
	defer mu.Unlock()

	globalConfig = config
	initialized = true
	throttleCache = make(map[string]time.Time)

	if config.Enabled {
		slog.Info("Alerting initialized",
			"service", config.ServiceName,
			"channels", len(config.Channels),
		)
	} else {
		slog.Info("Alerting disabled")
	}

	return nil
}

// Shutdown cleanly shuts down alerting.
func Shutdown() error {
	mu.Lock()
	defer mu.Unlock()

	if initialized && globalConfig.Enabled {
		slog.Info("Alerting shutdown")
	}

	initialized = false
	throttleCache = nil
	return nil
}

// SendAlert sends an alert through all enabled channels.
func SendAlert(ctx context.Context, alert Alert) {
	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	config := globalConfig
	mu.RUnlock()

	if !enabled {
		return
	}

	// Check throttling
	if config.ThrottleWindow > 0 {
		fingerprint := generateAlertFingerprint(alert)
		if isThrottled(fingerprint, config.ThrottleWindow) {
			slog.Debug("Alert throttled", "title", alert.Title)
			return
		}
		updateThrottleCache(fingerprint)
	}

	// Send to all enabled channels
	for _, channel := range config.Channels {
		if !channel.Enabled {
			continue
		}

		switch channel.Type {
		case ChannelTypeLog:
			sendToLog(alert)
		case ChannelTypeSlack:
			sendToSlack(ctx, alert, channel.Config)
		case ChannelTypePagerDuty:
			sendToPagerDuty(ctx, alert, channel.Config)
		case ChannelTypeEmail:
			sendToEmail(ctx, alert, channel.Config)
		default:
			slog.Warn("Unknown channel type", "type", channel.Type)
		}
	}
}

// sendToLog sends alert to structured logs.
func sendToLog(alert Alert) {
	logAttrs := []any{
		"title", alert.Title,
		"message", alert.Message,
		"severity", alert.Severity,
	}

	if len(alert.Tags) > 0 {
		logAttrs = append(logAttrs, "tags", alert.Tags)
	}

	if len(alert.Details) > 0 {
		logAttrs = append(logAttrs, "details", alert.Details)
	}

	switch alert.Severity {
	case SeverityCritical, SeverityError:
		slog.Error("ALERT", logAttrs...)
	case SeverityWarning:
		slog.Warn("ALERT", logAttrs...)
	default:
		slog.Info("ALERT", logAttrs...)
	}
}

// sendToSlack sends alert to Slack webhook.
// For MVP, this is a placeholder. In production, use Slack API.
func sendToSlack(ctx context.Context, alert Alert, config map[string]string) {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		slog.Warn("Slack webhook URL not configured")
		return
	}

	slog.Info("Alert sent to Slack",
		"title", alert.Title,
		"severity", alert.Severity,
		"webhook", webhookURL,
	)

	// TODO: Implement actual Slack webhook POST
	// payload := map[string]any{
	//     "text": alert.Title,
	//     "attachments": []map[string]any{
	//         {
	//             "color": getSeverityColor(alert.Severity),
	//             "text":  alert.Message,
	//             "fields": buildSlackFields(alert),
	//         },
	//     },
	// }
	// http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
}

// sendToPagerDuty sends alert to PagerDuty.
// For MVP, this is a placeholder. In production, use PagerDuty Events API.
func sendToPagerDuty(ctx context.Context, alert Alert, config map[string]string) {
	integrationKey := config["integration_key"]
	if integrationKey == "" {
		slog.Warn("PagerDuty integration key not configured")
		return
	}

	slog.Info("Alert sent to PagerDuty",
		"title", alert.Title,
		"severity", alert.Severity,
	)

	// TODO: Implement actual PagerDuty Events API v2 call
	// https://api.pagerduty.com/incidents
}

// sendToEmail sends alert via email.
// For MVP, this is a placeholder.
func sendToEmail(ctx context.Context, alert Alert, config map[string]string) {
	recipients := config["recipients"]
	if recipients == "" {
		slog.Warn("Email recipients not configured")
		return
	}

	slog.Info("Alert sent via email",
		"title", alert.Title,
		"severity", alert.Severity,
		"recipients", recipients,
	)

	// TODO: Implement actual email sending
	// Use SMTP or email service API
}

// generateAlertFingerprint creates a unique fingerprint for throttling.
func generateAlertFingerprint(alert Alert) string {
	hash := sha256.New()
	hash.Write([]byte(alert.Title))
	hash.Write([]byte(alert.Message))
	hash.Write([]byte(alert.Severity))
	return hex.EncodeToString(hash.Sum(nil))
}

// isThrottled checks if an alert should be throttled.
func isThrottled(fingerprint string, windowSeconds int) bool {
	throttleMu.RLock()
	defer throttleMu.RUnlock()

	lastSent, exists := throttleCache[fingerprint]
	if !exists {
		return false
	}

	return time.Since(lastSent) < time.Duration(windowSeconds)*time.Second
}

// updateThrottleCache updates the last sent time for an alert.
func updateThrottleCache(fingerprint string) {
	throttleMu.Lock()
	defer throttleMu.Unlock()

	throttleCache[fingerprint] = time.Now()

	// Cleanup old entries (older than 1 hour)
	for fp, t := range throttleCache {
		if time.Since(t) > time.Hour {
			delete(throttleCache, fp)
		}
	}
}
