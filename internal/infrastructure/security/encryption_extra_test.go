package security

import (
	"encoding/base64"
	"testing"
)

// TestEncryptionService_Decrypt_ShortData tests decryption of data that's too short.
func TestEncryptionService_Decrypt_ShortData(t *testing.T) {
	key := "12345678901234567890123456789012" // 32 bytes
	service := NewEncryptionService(key)

	// Create base64-encoded data that's too short to contain a valid nonce
	shortData := base64.StdEncoding.EncodeToString([]byte("ab")) // Only 2 bytes, less than nonce size (12)

	_, err := service.Decrypt(shortData)
	if err == nil {
		t.Fatal("Expected error for too-short ciphertext")
	}
}

// TestEncryptionService_Decrypt_TamperedData tests decryption of tampered data.
func TestEncryptionService_Decrypt_TamperedData(t *testing.T) {
	key := "12345678901234567890123456789012" // 32 bytes
	service := NewEncryptionService(key)

	// First encrypt something valid
	ciphertext, err := service.Encrypt("original plaintext")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decode, tamper, re-encode
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	// Tamper with the last byte of the ciphertext
	decoded[len(decoded)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(decoded)

	_, err = service.Decrypt(tampered)
	if err == nil {
		t.Fatal("Expected error for tampered ciphertext")
	}
}

// TestEncryptionService_Encrypt_Success tests multiple encryptions.
func TestEncryptionService_Encrypt_Success(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz012345" // 32 bytes
	service := NewEncryptionService(key)

	plaintexts := []string{
		"short",
		"medium length string",
		"a much longer string with more content to encrypt and decrypt",
		"", // empty string
		"unicode: 你好世界",
	}

	for _, pt := range plaintexts {
		t.Run(pt, func(t *testing.T) {
			ct, err := service.Encrypt(pt)
			if err != nil {
				t.Fatalf("Encrypt(%q) error = %v", pt, err)
			}

			decrypted, err := service.Decrypt(ct)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if decrypted != pt {
				t.Errorf("Roundtrip failed: got %q, want %q", decrypted, pt)
			}
		})
	}
}

// TestEncryptionService_DifferentKeys tests that different keys produce different results.
func TestEncryptionService_DifferentKeys(t *testing.T) {
	key1 := "12345678901234567890123456789012" // 32 bytes
	key2 := "abcdefghijklmnopqrstuvwxyz012345" // 32 bytes

	service1 := NewEncryptionService(key1)
	service2 := NewEncryptionService(key2)

	plaintext := "secret data"

	ct1, err := service1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt with key1 error = %v", err)
	}

	// service2 should not be able to decrypt service1's ciphertext
	_, err = service2.Decrypt(ct1)
	if err == nil {
		t.Error("Expected error when decrypting with wrong key")
	}
}
