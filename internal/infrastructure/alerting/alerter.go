package alerting

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// ChannelType defines the alerting channel type.
type ChannelType string

const (
	ChannelTypeLog       ChannelType = "log"       // Log-based alerting
	ChannelTypeSlack     ChannelType = "slack"     // Slack webhooks
	ChannelTypePagerDuty ChannelType = "pagerduty" // PagerDuty integration
	ChannelTypeEmail     ChannelType = "email"     // Email notifications
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
func sendToSlack(ctx context.Context, alert Alert, config map[string]string) {
	webhookURL := config["webhook_url"]
	if webhookURL == "" {
		slog.Warn("Slack webhook URL not configured")
		return
	}

	payload := buildSlackPayload(alert, config)
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal Slack payload", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		slog.Error("Failed to create Slack request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send Slack alert", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Slack webhook returned non-OK status", "status", resp.StatusCode)
		return
	}

	slog.Info("Alert sent to Slack", "title", alert.Title, "severity", alert.Severity)
}

// buildSlackPayload constructs the Slack webhook JSON payload.
func buildSlackPayload(alert Alert, config map[string]string) map[string]any {
	fields := []map[string]any{
		{"title": "Severity", "value": string(alert.Severity), "short": true},
	}
	for k, v := range alert.Tags {
		fields = append(fields, map[string]any{"title": k, "value": v, "short": true})
	}

	payload := map[string]any{
		"text": fmt.Sprintf("[%s] %s", strings.ToUpper(string(alert.Severity)), alert.Title),
		"attachments": []map[string]any{
			{
				"color":  severityColor(alert.Severity),
				"text":   alert.Message,
				"fields": fields,
			},
		},
	}

	if channel := config["channel"]; channel != "" {
		payload["channel"] = channel
	}
	if username := config["username"]; username != "" {
		payload["username"] = username
	}

	return payload
}

// severityColor returns the Slack attachment color for a severity level.
func severityColor(s Severity) string {
	switch s {
	case SeverityCritical:
		return "#FF0000"
	case SeverityError:
		return "#CC0000"
	case SeverityWarning:
		return "#FFA500"
	default:
		return "#36A64F"
	}
}

// sendToPagerDuty sends alert to PagerDuty.
// For MVP, this is a placeholder. In production, use PagerDuty Events API.
func sendToPagerDuty(_ context.Context, alert Alert, config map[string]string) {
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

// sendToEmail sends alert via SMTP email.
func sendToEmail(_ context.Context, alert Alert, config map[string]string) {
	recipients := config["recipients"]
	if recipients == "" {
		slog.Warn("Email recipients not configured")
		return
	}

	smtpHost := config["smtp_host"]
	smtpPort := config["smtp_port"]
	if smtpHost == "" || smtpPort == "" {
		slog.Warn("Email SMTP host/port not configured")
		return
	}

	from := config["from"]
	if from == "" {
		slog.Warn("Email sender address not configured")
		return
	}

	to := strings.Split(recipients, ",")
	for i := range to {
		to[i] = strings.TrimSpace(to[i])
	}

	body := buildEmailBody(alert, from, to)

	addr := smtpHost + ":" + smtpPort
	var auth smtp.Auth
	if username := config["username"]; username != "" {
		auth = smtp.PlainAuth("", username, config["password"], smtpHost)
	}

	if err := smtp.SendMail(addr, auth, from, to, []byte(body)); err != nil {
		slog.Error("Failed to send email alert", "error", err, "recipients", recipients)
		return
	}

	slog.Info("Alert sent via email", "title", alert.Title, "severity", alert.Severity, "recipients", recipients)
}

// buildEmailBody constructs a MIME email message for an alert.
func buildEmailBody(alert Alert, from string, to []string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	b.WriteString(fmt.Sprintf("Subject: [%s] %s\r\n", strings.ToUpper(string(alert.Severity)), alert.Title))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf("Severity: %s\n", alert.Severity))
	b.WriteString(fmt.Sprintf("Title: %s\n", alert.Title))
	b.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))

	if len(alert.Tags) > 0 {
		b.WriteString("\nTags:\n")
		for k, v := range alert.Tags {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	if len(alert.Details) > 0 {
		b.WriteString("\nDetails:\n")
		for k, v := range alert.Details {
			b.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
	}
	return b.String()
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
