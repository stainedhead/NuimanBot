package security

import (
	"testing"
)

// TestEncryptionService_EncryptDecrypt tests basic encryption and decryption
func TestEncryptionService_EncryptDecrypt(t *testing.T) {
	key := "12345678901234567890123456789012" // exactly 32 bytes
	service := NewEncryptionService(key)

	plaintext := "my-secret-bot-token-12345"

	// Encrypt
	ciphertext, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Verify ciphertext is different from plaintext
	if ciphertext == plaintext {
		t.Error("ciphertext should be different from plaintext")
	}

	// Decrypt
	decrypted, err := service.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	// Verify decrypted matches original
	if decrypted != plaintext {
		t.Errorf("decrypted text doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

// TestEncryptionService_EncryptDeterministic tests that encryption is non-deterministic
func TestEncryptionService_EncryptNonDeterministic(t *testing.T) {
	key := "12345678901234567890123456789012" // exactly 32 bytes
	service := NewEncryptionService(key)

	plaintext := "my-secret-bot-token"

	// Encrypt twice
	ciphertext1, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext2, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Ciphertexts should be different (due to random IV)
	if ciphertext1 == ciphertext2 {
		t.Error("encryption should be non-deterministic (different IVs)")
	}

	// But both should decrypt to same plaintext
	decrypted1, err := service.Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	decrypted2, err := service.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("both ciphertexts should decrypt to original plaintext")
	}
}

// TestEncryptionService_EmptyString tests encrypting empty string
func TestEncryptionService_EmptyString(t *testing.T) {
	key := "12345678901234567890123456789012" // exactly 32 bytes
	service := NewEncryptionService(key)

	plaintext := ""

	// Encrypt
	ciphertext, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Decrypt
	decrypted, err := service.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted text doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

// TestEncryptionService_InvalidKey tests behavior with invalid key length
func TestEncryptionService_InvalidKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with invalid key length")
		}
	}()

	// This should panic because key is too short
	_ = NewEncryptionService("short-key")
}

// TestEncryptionService_DecryptInvalid tests decrypting invalid ciphertext
func TestEncryptionService_DecryptInvalid(t *testing.T) {
	key := "12345678901234567890123456789012" // exactly 32 bytes
	service := NewEncryptionService(key)

	invalidCiphertext := "not-a-valid-encrypted-string"

	// Should return error
	_, err := service.Decrypt(invalidCiphertext)
	if err == nil {
		t.Error("expected error when decrypting invalid ciphertext")
	}
}

// TestEncryptionService_LongText tests encrypting longer text
func TestEncryptionService_LongText(t *testing.T) {
	key := "12345678901234567890123456789012" // exactly 32 bytes
	service := NewEncryptionService(key)

	// Long bot token (test data, not real)
	plaintext := "test-bot-token-1234567890-EXAMPLE-NOT-REAL-ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	// Encrypt
	ciphertext, err := service.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Decrypt
	decrypted, err := service.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted text doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

// TestDeriveKey tests key derivation from passphrase
func TestDeriveKey(t *testing.T) {
	passphrase := "my-secure-passphrase"

	// Derive key
	key := DeriveKey(passphrase)

	// Key should be 32 bytes (256 bits)
	if len(key) != 32 {
		t.Errorf("expected key length 32, got %d", len(key))
	}

	// Deriving again with same passphrase should give same key
	key2 := DeriveKey(passphrase)
	if string(key) != string(key2) {
		t.Error("same passphrase should derive same key")
	}

	// Different passphrase should give different key
	key3 := DeriveKey("different-passphrase")
	if string(key) == string(key3) {
		t.Error("different passphrases should derive different keys")
	}
}
