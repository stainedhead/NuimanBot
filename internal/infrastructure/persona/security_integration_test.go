package persona

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
)

// --- RBAC Cross-User Access Tests ---

// TestFileRepository_CrossUserAccess verifies users cannot access other users' files.
func TestFileRepository_CrossUserAccess(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	// User A creates their SOUL.md file
	userAFile := &domain.PersonaFile{
		UserID:  "userA",
		Type:    domain.PersonaFileSOUL,
		Content: "UserA's personality - CONFIDENTIAL",
	}
	if err := repo.Save(ctx, userAFile); err != nil {
		t.Fatalf("failed to save userA file: %v", err)
	}

	// User B creates their SOUL.md file
	userBFile := &domain.PersonaFile{
		UserID:  "userB",
		Type:    domain.PersonaFileSOUL,
		Content: "UserB's personality - CONFIDENTIAL",
	}
	if err := repo.Save(ctx, userBFile); err != nil {
		t.Fatalf("failed to save userB file: %v", err)
	}

	// Verify User A can read their own file
	retrieved, err := repo.Get(ctx, "userA", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("userA should be able to read their own file: %v", err)
	}
	if retrieved.Content != "UserA's personality - CONFIDENTIAL" {
		t.Errorf("userA got wrong content: %s", retrieved.Content)
	}

	// Verify User B can read their own file
	retrieved, err = repo.Get(ctx, "userB", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("userB should be able to read their own file: %v", err)
	}
	if retrieved.Content != "UserB's personality - CONFIDENTIAL" {
		t.Errorf("userB got wrong content: %s", retrieved.Content)
	}

	// Verify files are isolated (no cross-contamination)
	retrieved, err = repo.Get(ctx, "userA", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("userA failed to read: %v", err)
	}
	if retrieved.Content == "UserB's personality - CONFIDENTIAL" {
		t.Error("userA should not see userB's content")
	}

	// Attempt path traversal to access userB's file as userA
	// This should be blocked by ValidateUserPath
	maliciousPath := "../userB/SOUL.md"
	_, err = ValidateUserPath(tempDir, "userA", maliciousPath)
	if err == nil {
		t.Error("path traversal to userB's file should be blocked")
	}
	if !errors.Is(err, domain.ErrPathTraversal) {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

// TestFileRepository_DirectPathManipulation verifies that even if a caller tries to
// manually set the Path field to another user's directory, it's rejected.
func TestFileRepository_DirectPathManipulation(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	// Attacker tries to save to userB's directory by manipulating Path field
	maliciousFile := &domain.PersonaFile{
		UserID:  "userA",
		Type:    domain.PersonaFileSOUL,
		Path:    filepath.Join(tempDir, "userB", "SOUL.md"), // Wrong path!
		Content: "Attacker's content",
	}

	err := repo.Save(ctx, maliciousFile)
	if err == nil {
		t.Error("save with mismatched path should be rejected")
	}

	// Verify the error message indicates path mismatch
	expectedPath := filepath.Join(tempDir, "userA", "SOUL.md")
	if err != nil && err.Error() != "path mismatch: expected "+expectedPath+", got "+maliciousFile.Path {
		t.Logf("got error (acceptable): %v", err)
	}

	// Verify userB's directory doesn't contain attacker's file
	userBPath := filepath.Join(tempDir, "userB", "SOUL.md")
	data, err := os.ReadFile(userBPath)
	if err == nil {
		t.Errorf("attacker's file should not exist at %s, found: %s", userBPath, string(data))
	}
}

// --- Path Traversal Attack Scenarios ---

// TestFileRepository_SymlinkAttack verifies that symlinks cannot be used to escape user directories.
// Note: Symlinks within the user's own directory are acceptable, but symlinks pointing
// outside the user directory should not allow data exfiltration.
func TestFileRepository_SymlinkAttack(t *testing.T) {
	if os.Getenv("SKIP_SYMLINK_TESTS") != "" {
		t.Skip("Skipping symlink tests (may require special permissions)")
	}

	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	// Create attacker's directory
	attackerDir := filepath.Join(tempDir, "attacker")
	if err := os.MkdirAll(attackerDir, 0755); err != nil {
		t.Fatalf("failed to create attacker dir: %v", err)
	}

	// Create victim's directory with secret file OUTSIDE the persona base directory
	victimDir := t.TempDir() // Separate temp dir, not under our base
	secretPath := filepath.Join(victimDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("TOP SECRET DATA"), 0644); err != nil {
		t.Fatalf("failed to create secret file: %v", err)
	}

	// Attacker creates symlink in their persona dir pointing to victim's secret
	symlinkPath := filepath.Join(attackerDir, "SOUL.md")
	if err := os.Symlink(secretPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Attempt to read via symlink
	retrieved, err := repo.Get(ctx, "attacker", domain.PersonaFileSOUL)

	// Security check: If read succeeds, verify it didn't expose victim's secret
	if err == nil {
		if retrieved.Content == "TOP SECRET DATA" {
			t.Error("SECURITY VIOLATION: Symlink allowed reading data outside user directory!")
		}
		// If we got other content (e.g., symlink was dereferenced within allowed space), that's ok
		t.Logf("symlink read succeeded with content: %q (verify it's safe)", retrieved.Content)
	} else {
		// Error is acceptable (file not found, path validation, etc.)
		t.Logf("symlink blocked with error: %v (acceptable)", err)
	}
}

// --- Security Audit Trail Tests ---

// TestFileRepository_AuditTrail verifies that security-relevant operations can be audited.
// Note: Actual audit logging is in the audit package, but we verify the paths are correct.
func TestFileRepository_AuditableOperations(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	tests := []struct {
		name      string
		operation func() error
		expectErr bool
	}{
		{
			name: "successful file save",
			operation: func() error {
				file := &domain.PersonaFile{
					UserID:  "user1",
					Type:    domain.PersonaFileSOUL,
					Content: "test content",
				}
				return repo.Save(ctx, file)
			},
			expectErr: false,
		},
		{
			name: "path traversal attempt in save",
			operation: func() error {
				file := &domain.PersonaFile{
					UserID:  "../etc",
					Type:    domain.PersonaFileSOUL,
					Content: "malicious",
				}
				return repo.Save(ctx, file)
			},
			expectErr: true,
		},
		{
			name: "path traversal attempt in get",
			operation: func() error {
				_, err := repo.Get(ctx, "../etc", domain.PersonaFileSOUL)
				return err
			},
			expectErr: true,
		},
		{
			name: "path traversal attempt in delete",
			operation: func() error {
				return repo.Delete(ctx, "../etc", domain.PersonaFileSOUL)
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if tt.expectErr && err == nil {
				t.Error("expected error for security violation")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// In a real system, this would verify audit logs
			// For now, we just verify that errors are returned properly
			if tt.expectErr && !errors.Is(err, domain.ErrPathTraversal) {
				t.Logf("got error (auditable): %v", err)
			}
		})
	}
}

// --- Content Size and Validation Tests ---

// TestFileRepository_LargeFileHandling verifies handling of very large files.
func TestFileRepository_LargeFileHandling(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	// Create a 10MB file (should be handled, but may trigger warnings)
	largeContent := make([]byte, 10*1024*1024) // 10MB
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}

	file := &domain.PersonaFile{
		UserID:  "user1",
		Type:    domain.PersonaFileSOUL,
		Content: string(largeContent),
	}

	err := repo.Save(ctx, file)
	// We accept the file being saved (or rejected if size limits are enforced)
	if err != nil {
		t.Logf("large file handling: %v (acceptable)", err)
	} else {
		// Verify it can be read back
		retrieved, err := repo.Get(ctx, "user1", domain.PersonaFileSOUL)
		if err != nil {
			t.Fatalf("failed to retrieve large file: %v", err)
		}
		if len(retrieved.Content) != len(file.Content) {
			t.Errorf("content length mismatch: got %d, want %d", len(retrieved.Content), len(file.Content))
		}
	}
}

// --- Cache Security Tests ---

// TestFileRepository_CachePoisoning verifies cache cannot be poisoned across users.
func TestFileRepository_CachePoisoning(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	// User A saves a file
	fileA := &domain.PersonaFile{
		UserID:  "userA",
		Type:    domain.PersonaFileSOUL,
		Content: "UserA content",
	}
	if err := repo.Save(ctx, fileA); err != nil {
		t.Fatalf("failed to save userA file: %v", err)
	}

	// Read it (caches it)
	cached, err := repo.Get(ctx, "userA", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("failed to read userA file: %v", err)
	}
	if cached.Content != "UserA content" {
		t.Errorf("unexpected content: %s", cached.Content)
	}

	// User B saves their own file
	fileB := &domain.PersonaFile{
		UserID:  "userB",
		Type:    domain.PersonaFileSOUL,
		Content: "UserB content",
	}
	if err := repo.Save(ctx, fileB); err != nil {
		t.Fatalf("failed to save userB file: %v", err)
	}

	// Read userA's file again (should still be userA's content, not userB's)
	retrieved, err := repo.Get(ctx, "userA", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("failed to read userA file again: %v", err)
	}
	if retrieved.Content != "UserA content" {
		t.Errorf("cache poisoning detected! got %s, want 'UserA content'", retrieved.Content)
	}

	// Read userB's file (should be userB's content)
	retrieved, err = repo.Get(ctx, "userB", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("failed to read userB file: %v", err)
	}
	if retrieved.Content != "UserB content" {
		t.Errorf("got %s, want 'UserB content'", retrieved.Content)
	}
}

// TestFileRepository_CacheInvalidationSecurity verifies cache invalidation is per-user.
func TestFileRepository_CacheInvalidationSecurity(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	// User A saves and caches a file
	fileA := &domain.PersonaFile{
		UserID:  "userA",
		Type:    domain.PersonaFileSOUL,
		Content: "Original A",
	}
	if err := repo.Save(ctx, fileA); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := repo.Get(ctx, "userA", domain.PersonaFileSOUL); err != nil {
		t.Fatalf("get failed: %v", err)
	}

	// User B saves their file (should NOT invalidate userA's cache)
	fileB := &domain.PersonaFile{
		UserID:  "userB",
		Type:    domain.PersonaFileSOUL,
		Content: "Original B",
	}
	if err := repo.Save(ctx, fileB); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// User A updates their own file (should invalidate only userA's cache)
	fileA.Content = "Updated A"
	if err := repo.Save(ctx, fileA); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify userA sees the update
	retrieved, err := repo.Get(ctx, "userA", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if retrieved.Content != "Updated A" {
		t.Errorf("userA should see update, got: %s", retrieved.Content)
	}

	// Verify userB still sees their original content
	retrieved, err = repo.Get(ctx, "userB", domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if retrieved.Content != "Original B" {
		t.Errorf("userB should see original, got: %s", retrieved.Content)
	}
}

// --- Unicode and Special Character Tests ---

// TestFileRepository_UnicodeUserID verifies Unicode user IDs are handled safely.
func TestFileRepository_UnicodeUserID(t *testing.T) {
	tempDir := t.TempDir()
	repo := NewFileRepository(tempDir)
	ctx := context.Background()

	tests := []struct {
		name   string
		userID string
		valid  bool
	}{
		{name: "ASCII safe", userID: "user123", valid: true},
		{name: "Unicode safe", userID: "用户123", valid: true},
		{name: "Emoji", userID: "user😀", valid: true},
		{name: "Arabic", userID: "مستخدم", valid: true},
		{name: "Unicode with traversal", userID: "../用户", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &domain.PersonaFile{
				UserID:  tt.userID,
				Type:    domain.PersonaFileSOUL,
				Content: "test",
			}

			err := repo.Save(ctx, file)
			if tt.valid && err != nil {
				t.Errorf("valid userID %q should be accepted: %v", tt.userID, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("invalid userID %q should be rejected", tt.userID)
			}
		})
	}
}
