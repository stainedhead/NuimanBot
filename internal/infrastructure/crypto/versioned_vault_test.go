package crypto_test

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
)

func TestVersionedVault_StoreAndRetrieve(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-credential"
	value := domain.NewSecureStringFromString("secret-value")

	// Store credential
	if err := vault.Store(ctx, key, value); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Retrieve credential
	retrieved, err := vault.Retrieve(ctx, key)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}

	if retrieved.Value() != value.Value() {
		t.Errorf("Retrieve() = %v, want %v", retrieved.Value(), value.Value())
	}
}

func TestVersionedVault_MultipleKeyVersions(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store with version 1 (current)
	key1 := "credential-v1"
	value1 := domain.NewSecureStringFromString("value-encrypted-with-v1")
	if err := vault.Store(ctx, key1, value1); err != nil {
		t.Fatalf("Store() v1 error = %v", err)
	}

	// Add a new key version
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 1)
	}
	if err := vault.AddKeyVersion(2, newKey); err != nil {
		t.Fatalf("AddKeyVersion() error = %v", err)
	}

	// Set new version as current
	vault.SetCurrentVersion(2)

	// Store with version 2 (new current)
	key2 := "credential-v2"
	value2 := domain.NewSecureStringFromString("value-encrypted-with-v2")
	if err := vault.Store(ctx, key2, value2); err != nil {
		t.Fatalf("Store() v2 error = %v", err)
	}

	// Should be able to retrieve both
	retrieved1, err := vault.Retrieve(ctx, key1)
	if err != nil {
		t.Fatalf("Retrieve() v1 error = %v", err)
	}
	if retrieved1.Value() != value1.Value() {
		t.Errorf("Retrieve() v1 = %v, want %v", retrieved1.Value(), value1.Value())
	}

	retrieved2, err := vault.Retrieve(ctx, key2)
	if err != nil {
		t.Fatalf("Retrieve() v2 error = %v", err)
	}
	if retrieved2.Value() != value2.Value() {
		t.Errorf("Retrieve() v2 = %v, want %v", retrieved2.Value(), value2.Value())
	}
}

func TestVersionedVault_GetKeyVersion(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store credential
	key := "test-key"
	value := domain.NewSecureStringFromString("test-value")
	if err := vault.Store(ctx, key, value); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Get version used for encryption
	version, err := vault.GetKeyVersion(ctx, key)
	if err != nil {
		t.Fatalf("GetKeyVersion() error = %v", err)
	}

	if version != 1 {
		t.Errorf("GetKeyVersion() = %v, want 1", version)
	}
}

func TestVersionedVault_ReEncrypt(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store with version 1
	key := "credential-to-reencrypt"
	value := domain.NewSecureStringFromString("original-value")
	if err := vault.Store(ctx, key, value); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Verify it's using version 1
	version, err := vault.GetKeyVersion(ctx, key)
	if err != nil {
		t.Fatalf("GetKeyVersion() error = %v", err)
	}
	if version != 1 {
		t.Errorf("Initial version = %v, want 1", version)
	}

	// Add version 2 and set as current
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 2)
	}
	if err := vault.AddKeyVersion(2, newKey); err != nil {
		t.Fatalf("AddKeyVersion() error = %v", err)
	}
	vault.SetCurrentVersion(2)

	// Re-encrypt the credential
	if err := vault.ReEncrypt(ctx, key); err != nil {
		t.Fatalf("ReEncrypt() error = %v", err)
	}

	// Verify it's now using version 2
	newVersion, err := vault.GetKeyVersion(ctx, key)
	if err != nil {
		t.Fatalf("GetKeyVersion() after re-encrypt error = %v", err)
	}
	if newVersion != 2 {
		t.Errorf("Version after re-encrypt = %v, want 2", newVersion)
	}

	// Verify value is still correct
	retrieved, err := vault.Retrieve(ctx, key)
	if err != nil {
		t.Fatalf("Retrieve() after re-encrypt error = %v", err)
	}
	if retrieved.Value() != value.Value() {
		t.Errorf("Retrieve() after re-encrypt = %v, want %v", retrieved.Value(), value.Value())
	}
}

func TestVersionedVault_ReEncryptAll(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store multiple credentials with version 1
	keys := []string{"key1", "key2", "key3"}
	values := []domain.SecureString{
		domain.NewSecureStringFromString("value1"),
		domain.NewSecureStringFromString("value2"),
		domain.NewSecureStringFromString("value3"),
	}

	for i, key := range keys {
		if err := vault.Store(ctx, key, values[i]); err != nil {
			t.Fatalf("Store() %s error = %v", key, err)
		}
	}

	// Add version 2 and set as current
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 3)
	}
	if err := vault.AddKeyVersion(2, newKey); err != nil {
		t.Fatalf("AddKeyVersion() error = %v", err)
	}
	vault.SetCurrentVersion(2)

	// Re-encrypt all credentials
	if err := vault.ReEncryptAll(ctx); err != nil {
		t.Fatalf("ReEncryptAll() error = %v", err)
	}

	// Verify all are using version 2
	for _, key := range keys {
		version, err := vault.GetKeyVersion(ctx, key)
		if err != nil {
			t.Fatalf("GetKeyVersion(%s) error = %v", key, err)
		}
		if version != 2 {
			t.Errorf("GetKeyVersion(%s) = %v, want 2", key, version)
		}
	}

	// Verify all values are still correct
	for i, key := range keys {
		retrieved, err := vault.Retrieve(ctx, key)
		if err != nil {
			t.Fatalf("Retrieve(%s) error = %v", key, err)
		}
		if retrieved.Value() != values[i].Value() {
			t.Errorf("Retrieve(%s) = %v, want %v", key, retrieved.Value(), values[i].Value())
		}
	}
}

func TestVersionedVault_UnknownKeyVersion(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	ctx := context.Background()

	// Store with version 1
	key := "test-key"
	value := domain.NewSecureStringFromString("test-value")
	if err := vault.Store(ctx, key, value); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Add version 2 and set as current so we can remove version 1
	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i + 1)
	}
	if err := vault.AddKeyVersion(2, newKey); err != nil {
		t.Fatalf("AddKeyVersion() error = %v", err)
	}
	vault.SetCurrentVersion(2)

	// Remove version 1 key (simulate key removed after grace period)
	if err := vault.RemoveKeyVersion(1); err != nil {
		t.Fatalf("RemoveKeyVersion() error = %v", err)
	}

	// Try to retrieve (should fail with unknown version error)
	_, err := vault.Retrieve(ctx, key)
	if err == nil {
		t.Fatal("Expected error for unknown key version")
	}
}

func TestVersionedVault_AddKeyVersion_DuplicateVersion(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	newKey := make([]byte, 32)
	for i := range newKey {
		newKey[i] = byte(i)
	}

	// Try to add version 1 again (already exists)
	err := vault.AddKeyVersion(1, newKey)
	if err == nil {
		t.Fatal("Expected error for duplicate version")
	}
}

func TestVersionedVault_SetCurrentVersion_UnknownVersion(t *testing.T) {
	vault, cleanup := createTestVersionedVault(t)
	defer cleanup()

	// Try to set non-existent version as current
	err := vault.SetCurrentVersion(99)
	if err == nil {
		t.Fatal("Expected error for unknown version")
	}
}

// Helper function
func createTestVersionedVault(t *testing.T) (*crypto.VersionedVault, func()) {
	t.Helper()

	tmpFile := t.TempDir() + "/test-versioned-vault.enc"

	// Create initial key (version 1)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	vault, err := crypto.NewVersionedVault(tmpFile, 1, key)
	if err != nil {
		t.Fatalf("NewVersionedVault() error = %v", err)
	}

	cleanup := func() {
		// Cleanup handled by t.TempDir()
	}

	return vault, cleanup
}
