package crypto_test

import (
	"bytes"
	"nuimanbot/internal/infrastructure/crypto"
	"testing"
)

// generateTestKey creates a 32-byte AES key for testing.
func generateTestKey() []byte {
	return bytes.Repeat([]byte("a"), 32) // A simple, repeatable key for tests
}

func TestEncryptDecrypt(t *testing.T) {
	key := generateTestKey()
	plaintext := []byte("This is a secret message.")

	// Test case 1: Successful encryption and decryption
	ciphertext, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Fatal("Encrypt returned empty ciphertext")
	}

	decrypted, err := crypto.Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data mismatch: got %s, want %s", decrypted, plaintext)
	}

	// Test case 2: Empty plaintext
	emptyPlaintext := []byte("")
	emptyCiphertext, err := crypto.Encrypt(emptyPlaintext, key)
	if err != nil {
		t.Fatalf("Encrypt with empty plaintext failed: %v", err)
	}
	emptyDecrypted, err := crypto.Decrypt(emptyCiphertext, key)
	if err != nil {
		t.Fatalf("Decrypt with empty ciphertext failed: %v", err)
	}
	if !bytes.Equal(emptyPlaintext, emptyDecrypted) {
		t.Errorf("Decrypted empty data mismatch: got %s, want %s", emptyDecrypted, emptyPlaintext)
	}
}

func TestEncryptInvalidKey(t *testing.T) {
	plaintext := []byte("secret")
	invalidKeys := [][]byte{
		bytes.Repeat([]byte("a"), 16), // 16 bytes
		bytes.Repeat([]byte("a"), 24), // 24 bytes
		bytes.Repeat([]byte("a"), 31), // 31 bytes
		bytes.Repeat([]byte("a"), 33), // 33 bytes
		{},                            // empty key
	}

	for _, k := range invalidKeys {
		_, err := crypto.Encrypt(plaintext, k)
		if err == nil {
			t.Errorf("Encrypt with invalid key length %d bytes should have failed", len(k))
		}
	}
}

func TestDecryptInvalidKey(t *testing.T) {
	key := generateTestKey()
	plaintext := []byte("secret")
	ciphertext, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed for setup: %v", err)
	}

	invalidKeys := [][]byte{
		bytes.Repeat([]byte("b"), 32), // Different 32-byte key
		bytes.Repeat([]byte("a"), 16), // Invalid length
	}

	for _, k := range invalidKeys {
		_, err := crypto.Decrypt(ciphertext, k)
		if err == nil {
			t.Errorf("Decrypt with invalid key length %d bytes or wrong key should have failed", len(k))
		}
	}
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	key := generateTestKey()
	plaintext := []byte("secret")
	ciphertext, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed for setup: %v", err)
	}

	// Corrupt a byte in the ciphertext
	corruptedCiphertext := make([]byte, len(ciphertext))
	copy(corruptedCiphertext, ciphertext)
	if len(corruptedCiphertext) > 5 { // Ensure there are enough bytes to corrupt
		corruptedCiphertext[5] = ^corruptedCiphertext[5] // Flip a bit
	} else {
		t.Skipf("Ciphertext too short to corrupt for test: %d bytes", len(corruptedCiphertext))
	}

	_, err = crypto.Decrypt(corruptedCiphertext, key)
	if err == nil {
		t.Error("Decrypt with corrupted ciphertext should have failed")
	}
}

func TestDecryptTooShortCiphertext(t *testing.T) {
	key := generateTestKey()
	_, err := crypto.Decrypt([]byte("short"), key)
	if err == nil {
		t.Error("Decrypt with too short ciphertext should have failed")
	}
}
