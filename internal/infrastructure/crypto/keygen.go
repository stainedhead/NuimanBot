package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const (
	// EncryptionKeyLength is the required length for AES-256 encryption keys.
	EncryptionKeyLength = 32

	// EncryptionKeyEnvVar is the environment variable name for the encryption key.
	EncryptionKeyEnvVar = "NUIMANBOT_ENCRYPTION_KEY"
)

// GenerateEncryptionKey generates a cryptographically secure random 32-byte key
// suitable for AES-256 encryption.
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, EncryptionKeyLength)
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}

// EncodeKeyToBase64 encodes a byte slice key to a base64 string for storage.
func EncodeKeyToBase64(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// SaveEncryptionKeyToEnv saves the encryption key to the .env file with appropriate warnings.
// If the .env file exists, it appends to it. If not, it creates a new one.
func SaveEncryptionKeyToEnv(envPath string, key []byte) error {
	encodedKey := EncodeKeyToBase64(key)

	// Build the content to append
	var content strings.Builder
	content.WriteString("\n")
	content.WriteString("# ========================================\n")
	content.WriteString("# AUTO-GENERATED ENCRYPTION KEY\n")
	content.WriteString("# ========================================\n")
	content.WriteString("# This key was automatically generated on first startup.\n")
	content.WriteString("# IMPORTANT: Keep this key safe! You need it to access encrypted credentials.\n")
	content.WriteString("# If you lose this key, you will lose access to all stored credentials.\n")
	content.WriteString("# \n")
	content.WriteString("# For production deployments, consider using a secrets manager instead.\n")
	content.WriteString("# ========================================\n")
	content.WriteString(fmt.Sprintf("%s=%s\n", EncryptionKeyEnvVar, encodedKey))

	// Open file for appending (create if doesn't exist)
	f, err := os.OpenFile(envPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(content.String())
	if err != nil {
		return fmt.Errorf("failed to write to .env file: %w", err)
	}

	return nil
}

// IsEncryptionKeySet checks if the encryption key environment variable is set.
func IsEncryptionKeySet() bool {
	return os.Getenv(EncryptionKeyEnvVar) != ""
}
