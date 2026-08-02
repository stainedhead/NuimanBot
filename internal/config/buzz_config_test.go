package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/config"
)

func TestBuzzConfig_YAMLUnmarshal(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
gateways:
  buzz:
    enabled: true
    private_key: "secret-nsec-hex"
    relays:
      - "wss://relay.example.com"
      - "wss://relay2.example.com"
    nip05: "agent@example.com"
    channel_ids:
      - "channel-uuid-1"
      - "channel-uuid-2"
    dm_policy: "pairing"
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.Gateways.Buzz.Enabled {
		t.Error("Expected Gateways.Buzz.Enabled true, got false")
	}
	if cfg.Gateways.Buzz.PrivateKey.Value() != "secret-nsec-hex" {
		t.Errorf("Expected PrivateKey %q, got %q", "secret-nsec-hex", cfg.Gateways.Buzz.PrivateKey.Value())
	}
	wantRelays := []string{"wss://relay.example.com", "wss://relay2.example.com"}
	if len(cfg.Gateways.Buzz.Relays) != len(wantRelays) {
		t.Fatalf("Expected %d relays, got %d", len(wantRelays), len(cfg.Gateways.Buzz.Relays))
	}
	for i, r := range wantRelays {
		if cfg.Gateways.Buzz.Relays[i] != r {
			t.Errorf("Relays[%d] = %q, want %q", i, cfg.Gateways.Buzz.Relays[i], r)
		}
	}
	if cfg.Gateways.Buzz.NIP05 != "agent@example.com" {
		t.Errorf("Expected NIP05 %q, got %q", "agent@example.com", cfg.Gateways.Buzz.NIP05)
	}
	wantChannels := []string{"channel-uuid-1", "channel-uuid-2"}
	if len(cfg.Gateways.Buzz.ChannelIDs) != len(wantChannels) {
		t.Fatalf("Expected %d channel IDs, got %d", len(wantChannels), len(cfg.Gateways.Buzz.ChannelIDs))
	}
	for i, c := range wantChannels {
		if cfg.Gateways.Buzz.ChannelIDs[i] != c {
			t.Errorf("ChannelIDs[%d] = %q, want %q", i, cfg.Gateways.Buzz.ChannelIDs[i], c)
		}
	}
	if cfg.Gateways.Buzz.DMPolicy != config.DMPolicyPairing {
		t.Errorf("Expected DMPolicy %q, got %q", config.DMPolicyPairing, cfg.Gateways.Buzz.DMPolicy)
	}
}

func TestBuzzConfig_DefaultsDisabled(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Gateways.Buzz.Enabled {
		t.Error("Expected Gateways.Buzz.Enabled to default to false when absent")
	}
}
