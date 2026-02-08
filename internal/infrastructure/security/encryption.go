package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// EncryptionService provides AES-256-GCM encryption/decryption for sensitive data.
// Used to encrypt bot tokens and other secrets at rest.
type EncryptionService struct {
	key []byte
}

// NewEncryptionService creates a new encryption service with the given key.
// Key must be exactly 32 bytes (256 bits) for AES-256.
// Panics if key length is invalid.
func NewEncryptionService(key string) *EncryptionService {
	keyBytes := []byte(key)
	if len(keyBytes) != 32 {
		panic(fmt.Sprintf("encryption key must be exactly 32 bytes, got %d bytes", len(keyBytes)))
	}

	return &EncryptionService{
		key: keyBytes,
	}
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64-encoded ciphertext.
// Format: base64(nonce + ciphertext + tag)
func (e *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		// Allow encrypting empty strings
		plaintext = ""
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for safe storage in JSON
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encoded, nil
}

// Decrypt decrypts base64-encoded ciphertext and returns plaintext.
func (e *EncryptionService) Decrypt(ciphertext string) (string, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimum length (nonce + at least one byte + tag)
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// DeriveKey derives a 32-byte encryption key from a passphrase using SHA-256.
// Used when encryption key is provided as a passphrase instead of a 32-byte string.
func DeriveKey(passphrase string) []byte {
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:]
}
