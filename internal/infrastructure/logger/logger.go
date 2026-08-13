package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Config defines logger configuration
type Config struct {
	Level  LogLevel
	Format string // "json" or "text"
	// Output is where log records are written. Nil preserves the historical
	// default of os.Stdout. The ACP entrypoint (cmd/nuimanbot/acp.go) sets
	// this to os.Stderr — stdout there is reserved exclusively for the ACP
	// JSON-RPC stream, and a stray log line on stdout would corrupt it.
	Output io.Writer
}

// Initialize sets up the global slog logger
func Initialize(cfg Config) {
	level := parseLogLevel(cfg.Level)

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level LogLevel) slog.Level {
	switch strings.ToLower(string(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
