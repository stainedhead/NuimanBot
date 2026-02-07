package crypto

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"nuimanbot/internal/domain"
)

// VersionedVault implements secret rotation support with multiple key versions.
// It wraps FileCredentialVault and adds version tracking for each encrypted value.
type VersionedVault struct {
	filePath       string
	keys           map[int][]byte // version -> encryption key
	currentVersion int
	vault          *FileCredentialVault
	mu             sync.RWMutex
}

// NewVersionedVault creates a new versioned vault with an initial key version.
func NewVersionedVault(filePath string, initialVersion int, initialKey []byte) (*VersionedVault, error) {
	if len(initialKey) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256")
	}

	vault, err := NewFileCredentialVault(filePath, initialKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create underlying vault: %w", err)
	}

	v := &VersionedVault{
		filePath:       filePath,
		keys:           make(map[int][]byte),
		currentVersion: initialVersion,
		vault:          vault,
	}

	v.keys[initialVersion] = initialKey

	return v, nil
}

// Store encrypts and stores a credential using the current key version.
// The version is prepended to the encrypted data.
func (v *VersionedVault) Store(ctx context.Context, key string, value domain.SecureString) error {
	v.mu.RLock()
	currentVer := v.currentVersion
	encKey, exists := v.keys[currentVer]
	v.mu.RUnlock()

	if !exists {
		return fmt.Errorf("current key version %d not found", currentVer)
	}

	// Encrypt with current version key
	encrypted, err := Encrypt([]byte(value.Value()), encKey)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Prepend version header
	versioned := prependVersion(currentVer, encrypted)

	// Store versioned data in underlying vault
	return v.vault.Store(ctx, key, domain.NewSecureString(versioned))
}

// Retrieve decrypts and retrieves a credential using the appropriate key version.
func (v *VersionedVault) Retrieve(ctx context.Context, key string) (domain.SecureString, error) {
	// Retrieve versioned data from underlying vault
	versionedData, err := v.vault.Retrieve(ctx, key)
	if err != nil {
		return domain.SecureString{}, err
	}

	// Extract version header and encrypted data
	version, encrypted, err := extractVersion([]byte(versionedData.Value()))
	if err != nil {
		return domain.SecureString{}, fmt.Errorf("version extraction failed: %w", err)
	}

	// Get encryption key for this version
	v.mu.RLock()
	encKey, exists := v.keys[version]
	v.mu.RUnlock()

	if !exists {
		return domain.SecureString{}, fmt.Errorf("key version %d not found (may have been removed)", version)
	}

	// Decrypt with version-specific key
	decrypted, err := Decrypt(encrypted, encKey)
	if err != nil {
		return domain.SecureString{}, fmt.Errorf("decryption failed: %w", err)
	}

	return domain.NewSecureString(decrypted), nil
}

// Delete removes a credential.
func (v *VersionedVault) Delete(ctx context.Context, key string) error {
	return v.vault.Delete(ctx, key)
}

// List returns all stored credential keys.
func (v *VersionedVault) List(ctx context.Context) ([]string, error) {
	return v.vault.List(ctx)
}

// RotateKey is not needed for VersionedVault as rotation is handled via AddKeyVersion/ReEncrypt.
func (v *VersionedVault) RotateKey(ctx context.Context) error {
	return fmt.Errorf("use AddKeyVersion() and ReEncrypt() for key rotation in VersionedVault")
}

// AddKeyVersion adds a new encryption key version.
// Returns an error if the version already exists.
func (v *VersionedVault) AddKeyVersion(version int, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes for AES-256")
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.keys[version]; exists {
		return fmt.Errorf("key version %d already exists", version)
	}

	v.keys[version] = key
	return nil
}

// SetCurrentVersion sets the current encryption version for new credentials.
// Returns an error if the version doesn't exist.
func (v *VersionedVault) SetCurrentVersion(version int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.keys[version]; !exists {
		return fmt.Errorf("key version %d does not exist", version)
	}

	v.currentVersion = version
	return nil
}

// RemoveKeyVersion removes an encryption key version.
// Be careful: credentials encrypted with this version will become unreadable.
func (v *VersionedVault) RemoveKeyVersion(version int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if version == v.currentVersion {
		return fmt.Errorf("cannot remove current key version")
	}

	delete(v.keys, version)
	return nil
}

// GetKeyVersion returns the key version used to encrypt a credential.
func (v *VersionedVault) GetKeyVersion(ctx context.Context, key string) (int, error) {
	// Retrieve versioned data
	versionedData, err := v.vault.Retrieve(ctx, key)
	if err != nil {
		return 0, err
	}

	// Extract version
	version, _, err := extractVersion([]byte(versionedData.Value()))
	if err != nil {
		return 0, fmt.Errorf("failed to extract version: %w", err)
	}

	return version, nil
}

// ReEncrypt re-encrypts a credential with the current key version.
func (v *VersionedVault) ReEncrypt(ctx context.Context, key string) error {
	// Retrieve the current value
	value, err := v.Retrieve(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to retrieve credential: %w", err)
	}

	// Store it again (will use current version)
	if err := v.Store(ctx, key, value); err != nil {
		return fmt.Errorf("failed to store re-encrypted credential: %w", err)
	}

	return nil
}

// ReEncryptAll re-encrypts all credentials with the current key version.
// This is useful after adding a new key version.
func (v *VersionedVault) ReEncryptAll(ctx context.Context) error {
	// Get all credential keys
	keys, err := v.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list credentials: %w", err)
	}

	// Re-encrypt each credential
	for _, key := range keys {
		if err := v.ReEncrypt(ctx, key); err != nil {
			return fmt.Errorf("failed to re-encrypt %s: %w", key, err)
		}
	}

	return nil
}

// Helper functions

// prependVersion prepends a 4-byte version number to encrypted data.
func prependVersion(version int, data []byte) []byte {
	versionBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(versionBytes, uint32(version))
	return append(versionBytes, data...)
}

// extractVersion extracts the version number and encrypted data.
func extractVersion(versionedData []byte) (int, []byte, error) {
	if len(versionedData) < 4 {
		return 0, nil, fmt.Errorf("versioned data too short")
	}

	version := int(binary.BigEndian.Uint32(versionedData[:4]))
	encrypted := versionedData[4:]

	return version, encrypted, nil
}
