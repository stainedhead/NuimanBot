package domain

import (
	"errors"
)

// Default paths for configuration, data, and logs
const (
	DefaultConfigPath = "./config"
	DefaultDataPath   = "./data"
	DefaultLogsPath   = "./logs"
)

// PathsConfig defines configurable paths for server operations.
// Supports container-friendly deployments with separate volumes.
type PathsConfig struct {
	Config string `yaml:"config" json:"config"` // Path to configuration directory
	Data   string `yaml:"data" json:"data"`     // Path to data directory (users, bots, conversations)
	Logs   string `yaml:"logs" json:"logs"`     // Path to logs directory
}

// Validate checks if the paths configuration is valid.
func (p *PathsConfig) Validate() error {
	if p.Config == "" {
		return errors.New("config path cannot be empty")
	}
	if p.Data == "" {
		return errors.New("data path cannot be empty")
	}
	if p.Logs == "" {
		return errors.New("logs path cannot be empty")
	}
	return nil
}

// WithDefaults initializes paths with default values if not set.
func (p *PathsConfig) WithDefaults() {
	if p.Config == "" {
		p.Config = DefaultConfigPath
	}
	if p.Data == "" {
		p.Data = DefaultDataPath
	}
	if p.Logs == "" {
		p.Logs = DefaultLogsPath
	}
}

// GatewayEnabledConfig provides common enabled flag for gateways.
type GatewayEnabledConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// IsEnabled returns whether the gateway is enabled.
func (g *GatewayEnabledConfig) IsEnabled() bool {
	return g.Enabled
}
