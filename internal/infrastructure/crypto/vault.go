package crypto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"nuimanbot/internal/domain" // Import for ErrNotFound and SecureString
	"os"
	"path/filepath"
	"sync"
)

// fileCredentialVault implements the security.CredentialVault interface.
// It stores encrypted credentials in a file on disk.
type FileCredentialVault struct {
	filePath       string
	encryptionKey  []byte
	mu             sync.RWMutex      // Protects access to credentialsMap and file operations
	credentialsMap map[string][]byte // Stores encrypted values
}

// NewFileCredentialVault creates a new file-based credential vault.
// The filePath specifies where the encrypted credentials will be stored.
// The encryptionKey must be 32 bytes for AES-256.
func NewFileCredentialVault(filePath string, encryptionKey []byte) (*FileCredentialVault, error) {
	if len(encryptionKey) != 32 {
		return nil, errors.New("encryption key must be 32 bytes for AES-256")
	}

	vault := &FileCredentialVault{
		filePath:       filePath,
		encryptionKey:  encryptionKey,
		credentialsMap: make(map[string][]byte),
	}

	// Load existing credentials if file exists
	err := vault.loadFromFile()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load credentials from file: %w", err)
	}

	return vault, nil
}

// Store securely stores a credential.
func (v *FileCredentialVault) Store(ctx context.Context, key string, value domain.SecureString) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	encryptedValue, err := Encrypt([]byte(value.Value()), v.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential: %w", err)
	}
	v.credentialsMap[key] = encryptedValue

	return v.saveToFile()
}

// Retrieve retrieves a credential.
func (v *FileCredentialVault) Retrieve(ctx context.Context, key string) (domain.SecureString, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	encryptedValue, ok := v.credentialsMap[key]
	if !ok {
		return domain.SecureString{}, domain.ErrNotFound
	}

	decryptedValue, err := Decrypt(encryptedValue, v.encryptionKey)
	if err != nil {
		return domain.SecureString{}, fmt.Errorf("failed to decrypt credential: %w", err)
	}

	return domain.NewSecureString(decryptedValue), nil
}

// Delete removes a credential.
func (v *FileCredentialVault) Delete(ctx context.Context, key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, ok := v.credentialsMap[key]; !ok {
		return domain.ErrNotFound
	}

	delete(v.credentialsMap, key)
	return v.saveToFile()
}

// RotateKey is not implemented for this file-based MVP vault.
// It would involve re-encrypting all stored credentials with a new key.
func (v *FileCredentialVault) RotateKey(ctx context.Context) error {
	return errors.New("key rotation not implemented for file-based vault MVP")
}

// List returns all stored credential keys (not values).
func (v *FileCredentialVault) List(ctx context.Context) ([]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	keys := make([]string, 0, len(v.credentialsMap))
	for key := range v.credentialsMap {
		keys = append(keys, key)
	}
	return keys, nil
}

// saveToFile encrypts the entire credentials map and writes it to disk.
func (v *FileCredentialVault) saveToFile() error {
	// Marshal the map of encrypted values to JSON
	jsonData, err := json.Marshal(v.credentialsMap)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials map: %w", err)
	}

	// Encrypt the JSON data (credentials map)
	encryptedData, err := Encrypt(jsonData, v.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials file content: %w", err)
	}

	// Ensure the directory exists
	dir := filepath.Dir(v.filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory for vault file: %w", err)
	}

	// Write the encrypted data to the file
	err = os.WriteFile(v.filePath, encryptedData, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write encrypted credentials to file: %w", err)
	}
	return nil
}

// loadFromFile reads the encrypted credentials file from disk and decrypts it.
func (v *FileCredentialVault) loadFromFile() error {
	data, err := os.ReadFile(v.filePath)
	if err != nil {
		return err // Return directly, os.IsNotExist check will be done by caller
	}

	decryptedData, err := Decrypt(data, v.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt credentials file: %w", err)
	}

	// Unmarshal the JSON data back into the map
	err = json.Unmarshal(decryptedData, &v.credentialsMap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal credentials map: %w", err)
	}
	return nil
}
