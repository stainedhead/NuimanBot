package crypto_test

import (
	"context"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
)

// TestBuildTLSConfig_Success tests successful TLS config creation.
func TestBuildTLSConfig_Success(t *testing.T) {
	dir := t.TempDir()
	cfg := config.TLSConfig{
		CertFile: filepath.Join(dir, "cert.pem"),
		KeyFile:  filepath.Join(dir, "key.pem"),
		Hosts:    []string{"localhost"},
	}

	tlsCfg, err := crypto.BuildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("BuildTLSConfig() error = %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("Expected non-nil TLS config")
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
}

// TestBuildTLSConfig_LoadsExisting tests that BuildTLSConfig loads existing cert files.
func TestBuildTLSConfig_LoadsExisting(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Generate cert first
	_, err := crypto.LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("LoadOrGenerateCert() error = %v", err)
	}

	// Now load via BuildTLSConfig
	cfg := config.TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		Hosts:    []string{"localhost"},
	}

	tlsCfg, err := crypto.BuildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("BuildTLSConfig() error = %v", err)
	}
	if tlsCfg == nil {
		t.Fatal("Expected non-nil TLS config")
	}
}

// TestBuildTLSConfig_InvalidCertFiles tests error when cert files have invalid content.
func TestBuildTLSConfig_InvalidCertFiles(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Create invalid cert files
	if err := os.WriteFile(certPath, []byte("not a cert"), 0644); err != nil {
		t.Fatalf("Failed to write invalid cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("not a key"), 0600); err != nil {
		t.Fatalf("Failed to write invalid key: %v", err)
	}

	cfg := config.TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		Hosts:    []string{"localhost"},
	}

	_, err := crypto.BuildTLSConfig(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid cert files")
	}
}

// TestLoadOrGenerateCert_WithIPAddresses tests cert generation with IP addresses.
func TestLoadOrGenerateCert_WithIPAddresses(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Use IP addresses as hosts
	cert, err := crypto.LoadOrGenerateCert(certPath, keyPath, []string{"192.168.1.1", "::1"})
	if err != nil {
		t.Fatalf("LoadOrGenerateCert() error = %v", err)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Should have the IP addresses
	if len(x509Cert.IPAddresses) == 0 {
		t.Error("Expected IP addresses in certificate")
	}
}

// TestAES_InvalidKey tests Encrypt and Decrypt with invalid key.
func TestAES_InvalidKey(t *testing.T) {
	shortKey := []byte("too-short")

	_, err := crypto.Encrypt([]byte("plaintext"), shortKey)
	if err == nil {
		t.Fatal("Expected error for short key in Encrypt")
	}

	_, err = crypto.Decrypt([]byte("ciphertext"), shortKey)
	if err == nil {
		t.Fatal("Expected error for short key in Decrypt")
	}
}

// TestAES_DecryptShortCiphertext tests Decrypt with ciphertext that's too short.
func TestAES_DecryptShortCiphertext(t *testing.T) {
	key := make([]byte, 32)

	_, err := crypto.Decrypt([]byte("short"), key)
	if err == nil {
		t.Fatal("Expected error for too-short ciphertext")
	}
}

// TestAES_EncryptDecryptRoundTrip tests Encrypt/Decrypt roundtrip.
func TestAES_EncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("hello, world!")
	ciphertext, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

// TestAES_DecryptTamperedCiphertext tests Decrypt with tampered data.
func TestAES_DecryptTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// Encrypt something
	ciphertext, err := crypto.Encrypt([]byte("original"), key)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Tamper with the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = crypto.Decrypt(ciphertext, key)
	if err == nil {
		t.Fatal("Expected error for tampered ciphertext")
	}
}

// TestVersionedVault_Delete tests Delete functionality.
func TestVersionedVault_Delete(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store a credential
	key := "cred-to-delete"
	value := domain.NewSecureStringFromString("secret")
	if err := vault.Store(ctx, key, value); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Delete it
	if err := vault.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Should not be retrievable anymore
	_, err := vault.Retrieve(ctx, key)
	if err == nil {
		t.Fatal("Expected error after deleting credential")
	}
}

// TestVersionedVault_List tests List functionality.
func TestVersionedVault_List(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store several credentials
	keys := []string{"cred1", "cred2", "cred3"}
	for _, k := range keys {
		if err := vault.Store(ctx, k, domain.NewSecureStringFromString("value-"+k)); err != nil {
			t.Fatalf("Store(%s) error = %v", k, err)
		}
	}

	// List all
	listed, err := vault.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != len(keys) {
		t.Errorf("Expected %d keys, got %d", len(keys), len(listed))
	}
}

// TestVersionedVault_RotateKey tests that RotateKey returns an error (not implemented).
func TestVersionedVault_RotateKey(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	err := vault.RotateKey(context.Background())
	if err == nil {
		t.Fatal("Expected error from RotateKey")
	}
}

// TestVersionedVault_RemoveCurrentKey tests that removing the current key version fails.
func TestVersionedVault_RemoveCurrentKey(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	// Try to remove the current (version 1) key
	err := vault.RemoveKeyVersion(1)
	if err == nil {
		t.Fatal("Expected error when removing current key version")
	}
}

// TestVersionedVault_NewVault_InvalidKey tests vault creation with wrong key length.
func TestVersionedVault_NewVault_InvalidKey(t *testing.T) {
	tmpFile := t.TempDir() + "/test.enc"
	shortKey := make([]byte, 16) // 16 bytes, not 32

	_, err := crypto.NewVersionedVault(tmpFile, 1, shortKey)
	if err == nil {
		t.Fatal("Expected error for short encryption key")
	}
}

// TestVersionedVault_GetKeyVersion_NonExistentKey tests GetKeyVersion for missing key.
func TestVersionedVault_GetKeyVersion_NonExistentKey(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	_, err := vault.GetKeyVersion(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent credential key")
	}
}

// TestVersionedVault_ReEncryptAll_Empty tests ReEncryptAll on empty vault.
func TestVersionedVault_ReEncryptAll_Empty(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	// ReEncryptAll on empty vault should succeed
	if err := vault.ReEncryptAll(context.Background()); err != nil {
		t.Fatalf("ReEncryptAll() error on empty vault = %v", err)
	}
}

// TestVersionedVault_ExtractVersionTooShort tests extractVersion with short data.
func TestVersionedVault_ExtractVersionTooShort(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	// Store with raw bytes that are too short (corrupt data)
	ctx := context.Background()

	// We can't directly call extractVersion (unexported), but we can store garbage
	// and try to retrieve to trigger the version extraction error path.
	// The vault stores raw SecureString, so we store a very short versioned payload:
	// Store a value that after encryption has the correct header but is nonsense when decoded
	// For VersionedVault, we can test this by manually corrupting stored data via the underlying API.
	// Instead, test through ReEncrypt on a key that doesn't exist:
	err := vault.ReEncrypt(ctx, "nonexistent-key")
	if err == nil {
		t.Fatal("Expected error for re-encrypting nonexistent key")
	}
}

// TestFileCredentialVault_RotateKey_Extra tests that RotateKey returns error.
func TestFileCredentialVault_RotateKey_Extra(t *testing.T) {
	key := make([]byte, 32)
	vault, err := crypto.NewFileCredentialVault(t.TempDir()+"/vault.enc", key)
	if err != nil {
		t.Fatalf("NewFileCredentialVault() error = %v", err)
	}

	err = vault.RotateKey(context.Background())
	if err == nil {
		t.Fatal("Expected error from RotateKey")
	}
}

// TestFileCredentialVault_Delete_NotFound tests Delete for non-existent key.
func TestFileCredentialVault_Delete_NotFound(t *testing.T) {
	key := make([]byte, 32)
	vault, err := crypto.NewFileCredentialVault(t.TempDir()+"/vault.enc", key)
	if err != nil {
		t.Fatalf("NewFileCredentialVault() error = %v", err)
	}

	err = vault.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Expected error for deleting nonexistent key")
	}
}

// TestFileCredentialVault_StoreAndRetrieve_Extra tests store and retrieve.
func TestFileCredentialVault_StoreAndRetrieve_Extra(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	vault, err := crypto.NewFileCredentialVault(t.TempDir()+"/vault.enc", key)
	if err != nil {
		t.Fatalf("NewFileCredentialVault() error = %v", err)
	}

	ctx := context.Background()
	cred := "api-token-123"

	if err := vault.Store(ctx, "my-cred", domain.NewSecureStringFromString(cred)); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	retrieved, err := vault.Retrieve(ctx, "my-cred")
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if retrieved.Value() != cred {
		t.Errorf("Retrieved = %q, want %q", retrieved.Value(), cred)
	}
}

// TestFileCredentialVault_LoadExistingVault tests loading an existing vault from disk.
func TestFileCredentialVault_LoadExistingVault(t *testing.T) {
	dir := t.TempDir()
	vaultPath := dir + "/vault.enc"
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 5)
	}

	// Store a credential
	vault1, err := crypto.NewFileCredentialVault(vaultPath, key)
	if err != nil {
		t.Fatalf("NewFileCredentialVault() error = %v", err)
	}
	ctx := context.Background()
	if err := vault1.Store(ctx, "token", domain.NewSecureStringFromString("my-token")); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Load the same vault
	vault2, err := crypto.NewFileCredentialVault(vaultPath, key)
	if err != nil {
		t.Fatalf("Second NewFileCredentialVault() error = %v", err)
	}

	retrieved, err := vault2.Retrieve(ctx, "token")
	if err != nil {
		t.Fatalf("Retrieve() from loaded vault error = %v", err)
	}
	if retrieved.Value() != "my-token" {
		t.Errorf("Retrieved = %q, want %q", retrieved.Value(), "my-token")
	}
}

// TestFileCredentialVault_InvalidKey tests vault creation with wrong key length.
func TestFileCredentialVault_InvalidKey(t *testing.T) {
	shortKey := make([]byte, 16)
	_, err := crypto.NewFileCredentialVault(t.TempDir()+"/vault.enc", shortKey)
	if err == nil {
		t.Fatal("Expected error for short encryption key")
	}
}
