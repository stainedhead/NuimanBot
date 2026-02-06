package crypto_test

import (
	"context"
	"errors" // Added errors to imports

	"nuimanbot/internal/domain" // Import for ErrNotFound, SecureString, and NewSecureStringFromString
	"nuimanbot/internal/infrastructure/crypto"
	"os"
	"path/filepath"
	"testing"
)

// setupVault creates a temporary file path and a new vault instance for testing.
func setupVault(t *testing.T) (tempDir string, vault *crypto.FileCredentialVault, key []byte) {

	var e error
	tempDir, e = os.MkdirTemp("", "nuimanbot-vault-test-")
	if e != nil {
		t.Fatalf("Failed to create temp dir: %v", e)
	}
	filePath := filepath.Join(tempDir, "credentials.enc")
	key = generateTestKey() // Using the helper from aes_test.go

	vault, e = crypto.NewFileCredentialVault(filePath, key)
	if e != nil {
		t.Fatalf("Failed to create new vault: %v", e)
	}
	return tempDir, vault, key
}

// cleanupVault removes the temporary directory created by setupVault.
func cleanupVault(t *testing.T, tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		// Log the error during cleanup, but don't fail the test
		t.Logf("cleanupVault: failed to remove temporary directory %s: %v", tempDir, err)
	}
}

func TestFileCredentialVault_StoreAndRetrieve(t *testing.T) {
	tempDir, vault, _ := setupVault(t)
	defer cleanupVault(t, tempDir)

	ctx := context.Background()
	testKey := "my_api_key"
	testValue := domain.NewSecureStringFromString("supersecret123")

	err := vault.Store(ctx, testKey, testValue)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	retrievedValue, err := vault.Retrieve(ctx, testKey)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}

	if retrievedValue.Value() != testValue.Value() {
		t.Errorf("Retrieved value mismatch: got %s, want %s", retrievedValue.Value(), testValue.Value())
	}

	// Verify non-existent key
	_, err = vault.Retrieve(ctx, "non_existent_key")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Retrieve non-existent key: got %v, want ErrNotFound", err)
	}
}

func TestFileCredentialVault_Delete(t *testing.T) {
	tempDir, vault, _ := setupVault(t)
	defer cleanupVault(t, tempDir)

	ctx := context.Background()
	testKey := "key_to_delete"
	testValue := domain.NewSecureStringFromString("value_to_delete")

	if err := vault.Store(ctx, testKey, testValue); err != nil {
		t.Fatalf("Store failed during setup for delete test: %v", err)
	}

	err := vault.Delete(ctx, testKey)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = vault.Retrieve(ctx, testKey)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Retrieve after delete: got %v, want ErrNotFound", err)
	}

	// Try deleting non-existent key
	err = vault.Delete(ctx, "non_existent")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Delete non-existent key: got %v, want ErrNotFound", err)
	}
}

func TestFileCredentialVault_Persistence(t *testing.T) {
	tempDir, vault, key := setupVault(t)
	defer cleanupVault(t, tempDir)

	ctx := context.Background()
	testKey := "persisted_key"
	testValue := domain.NewSecureStringFromString("persisted_value")

	err := vault.Store(ctx, testKey, testValue)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Simulate application restart by creating a new vault instance with the same file and key
	newVault, err := crypto.NewFileCredentialVault(filepath.Join(tempDir, "credentials.enc"), key)
	if err != nil {
		t.Fatalf("Failed to create new vault after initial store: %v", err)
	}

	retrievedValue, err := newVault.Retrieve(ctx, testKey)
	if err != nil {
		t.Fatalf("Retrieve from new vault failed: %v", err)
	}

	if retrievedValue.Value() != testValue.Value() {
		t.Errorf("Persisted value mismatch: got %s, want %s", retrievedValue.Value(), testValue.Value())
	}
}

func TestFileCredentialVault_LoadFromFileNotExist(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nuimanbot-vault-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanupVault(t, tempDir)

	filePath := filepath.Join(tempDir, "non_existent_credentials.enc")
	key := generateTestKey()

	vault, err := crypto.NewFileCredentialVault(filePath, key)
	if err != nil {
		t.Fatalf("Failed to create vault for non-existent file: %v", err)
	}
	keys, err := vault.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("New vault from non-existent file should be empty, but found %d items", len(keys))
	}
}

func TestFileCredentialVault_CorruptedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nuimanbot-vault-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer cleanupVault(t, tempDir)

	filePath := filepath.Join(tempDir, "corrupted_credentials.enc")
	key := generateTestKey()

	// Write some corrupted data
	if err := os.WriteFile(filePath, []byte("this is not encrypted json"), 0o600); err != nil {
		t.Fatalf("Failed to write corrupted file for test: %v", err)
	}

	var vaultErr error // Declare err here for this scope
	_, vaultErr = crypto.NewFileCredentialVault(filePath, key)
	if vaultErr == nil {
		t.Error("Loading from corrupted file should have failed")
	}
}

func TestFileCredentialVault_List(t *testing.T) {
	tempDir, vault, _ := setupVault(t)
	defer cleanupVault(t, tempDir)

	ctx := context.Background()
	if err := vault.Store(ctx, "key1", domain.NewSecureStringFromString("val1")); err != nil {
		t.Fatalf("Store failed for key1 during list test setup: %v", err)
	}
	if err := vault.Store(ctx, "key2", domain.NewSecureStringFromString("val2")); err != nil {
		t.Fatalf("Store failed for key2 during list test setup: %v", err)
	}

	keys, err := vault.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	found1 := false
	found2 := false
	for _, k := range keys {
		if k == "key1" {
			found1 = true
		}
		if k == "key2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("Expected keys 'key1' and 'key2' not found in list: %v", keys)
	}
}

func TestFileCredentialVault_RotateKey(t *testing.T) {
	tempDir, vault, _ := setupVault(t)
	defer cleanupVault(t, tempDir)

	ctx := context.Background()
	err := vault.RotateKey(ctx)
	if err == nil {
		t.Error("RotateKey should return 'not implemented' error")
	}
}
