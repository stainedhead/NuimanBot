package crypto

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey failed: %v", err)
	}

	// Key should be exactly 32 bytes
	if len(key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key))
	}

	// Key should not be all zeros
	allZeros := true
	for _, b := range key {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("Generated key should not be all zeros")
	}
}

func TestGenerateEncryptionKey_UniqueKeys(t *testing.T) {
	// Generate multiple keys and ensure they're different
	key1, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate key1: %v", err)
	}

	key2, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate key2: %v", err)
	}

	// Keys should be different
	if string(key1) == string(key2) {
		t.Error("Two generated keys should be different")
	}
}

func TestEncodeKeyToBase64(t *testing.T) {
	key := []byte("12345678901234567890123456789012") // 32 bytes

	encoded := EncodeKeyToBase64(key)

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Errorf("Encoded key is not valid base64: %v", err)
	}

	// Decoded should match original
	if string(decoded) != string(key) {
		t.Error("Decoded key does not match original")
	}
}

func TestSaveEncryptionKeyToEnv(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	key := []byte("12345678901234567890123456789012") // 32 bytes

	// Save key to .env
	err := SaveEncryptionKeyToEnv(envPath, key)
	if err != nil {
		t.Fatalf("SaveEncryptionKeyToEnv failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Error(".env file was not created")
	}

	// Verify file contains the key
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}

	expectedKey := base64.StdEncoding.EncodeToString(key)
	if !strings.Contains(string(content), "NUIMANBOT_ENCRYPTION_KEY="+expectedKey) {
		t.Errorf(".env file does not contain expected key. Content:\n%s", content)
	}

	// Verify file contains warning comments
	if !strings.Contains(string(content), "AUTO-GENERATED") {
		t.Error(".env file should contain AUTO-GENERATED warning")
	}
}

func TestSaveEncryptionKeyToEnv_AppendsToExisting(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	// Create existing .env with some content
	existingContent := "SOME_VAR=value\nANOTHER_VAR=123\n"
	if err := os.WriteFile(envPath, []byte(existingContent), 0600); err != nil {
		t.Fatalf("Failed to create existing .env: %v", err)
	}

	key := []byte("12345678901234567890123456789012") // 32 bytes

	// Save key to .env
	err := SaveEncryptionKeyToEnv(envPath, key)
	if err != nil {
		t.Fatalf("SaveEncryptionKeyToEnv failed: %v", err)
	}

	// Verify file contains both old and new content
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}

	// Should preserve existing content
	if !strings.Contains(string(content), "SOME_VAR=value") {
		t.Error(".env file should preserve existing content")
	}

	// Should add new key
	expectedKey := base64.StdEncoding.EncodeToString(key)
	if !strings.Contains(string(content), "NUIMANBOT_ENCRYPTION_KEY="+expectedKey) {
		t.Error(".env file should contain new encryption key")
	}
}

// TestFirstRunEncryptionKeyRoundTrip reproduces the first-run flow: a key is
// generated, base64-encoded for .env storage exactly as SaveEncryptionKeyToEnv
// does, then read back from the environment variable string (as main.go does
// on subsequent access via config) and must decode to the original 32-byte
// raw key so that vault construction succeeds.
func TestFirstRunEncryptionKeyRoundTrip(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey failed: %v", err)
	}

	// Simulate what gets written to .env / NUIMANBOT_ENCRYPTION_KEY and what
	// gets read back from the environment on the next call to config.LoadConfig().
	envValue := EncodeKeyToBase64(key)

	decoded, err := DecodeKeyFromBase64(envValue)
	if err != nil {
		t.Fatalf("DecodeKeyFromBase64 failed: %v", err)
	}

	if len(decoded) != EncryptionKeyLength {
		t.Fatalf("Expected decoded key length %d, got %d", EncryptionKeyLength, len(decoded))
	}

	if string(decoded) != string(key) {
		t.Error("Decoded key does not match originally generated key")
	}
}

func TestIsEncryptionKeySet(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "Key is set",
			envValue: "some-key-value",
			expected: true,
		},
		{
			name:     "Key is empty",
			envValue: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set or unset the env var
			if tt.envValue != "" {
				t.Setenv("NUIMANBOT_ENCRYPTION_KEY", tt.envValue)
			} else {
				os.Unsetenv("NUIMANBOT_ENCRYPTION_KEY")
			}

			result := IsEncryptionKeySet()
			if result != tt.expected {
				t.Errorf("Expected IsEncryptionKeySet() = %v, got %v", tt.expected, result)
			}
		})
	}
}
