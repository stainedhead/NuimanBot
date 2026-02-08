package domain

import (
	"testing"
)

// TestPathsConfig tests the PathsConfig validation
func TestPathsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  PathsConfig
		wantErr bool
	}{
		{
			name: "valid paths",
			config: PathsConfig{
				Config: "./config",
				Data:   "./data",
				Logs:   "./logs",
			},
			wantErr: false,
		},
		{
			name: "empty config path",
			config: PathsConfig{
				Config: "",
				Data:   "./data",
				Logs:   "./logs",
			},
			wantErr: true,
		},
		{
			name: "empty data path",
			config: PathsConfig{
				Config: "./config",
				Data:   "",
				Logs:   "./logs",
			},
			wantErr: true,
		},
		{
			name: "empty logs path",
			config: PathsConfig{
				Config: "./config",
				Data:   "./data",
				Logs:   "",
			},
			wantErr: true,
		},
		{
			name: "absolute paths",
			config: PathsConfig{
				Config: "/etc/nuimanbot/config",
				Data:   "/var/lib/nuimanbot",
				Logs:   "/var/log/nuimanbot",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PathsConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPathsConfig_WithDefaults tests default path initialization
func TestPathsConfig_WithDefaults(t *testing.T) {
	config := PathsConfig{}
	config.WithDefaults()

	if config.Config != DefaultConfigPath {
		t.Errorf("expected config path %s, got %s", DefaultConfigPath, config.Config)
	}
	if config.Data != DefaultDataPath {
		t.Errorf("expected data path %s, got %s", DefaultDataPath, config.Data)
	}
	if config.Logs != DefaultLogsPath {
		t.Errorf("expected logs path %s, got %s", DefaultLogsPath, config.Logs)
	}
}

// TestGatewayEnabledConfig tests gateway enabled configuration
func TestGatewayEnabledConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GatewayEnabledConfig{Enabled: tt.enabled}
			if got := config.IsEnabled(); got != tt.want {
				t.Errorf("GatewayEnabledConfig.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
