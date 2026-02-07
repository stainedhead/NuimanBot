package common

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathValidator_ValidPath(t *testing.T) {
	allowedDirs := []string{"/tmp/workspace", "/home/user/projects"}
	validator := NewPathValidator(allowedDirs)

	testCases := []string{
		"/tmp/workspace/file.txt",
		"/tmp/workspace/subdir/file.txt",
		"/home/user/projects/myproject/src/main.go",
	}

	for _, path := range testCases {
		err := validator.ValidatePath(path)
		assert.NoError(t, err, "Path should be allowed: %s", path)

		allowed := validator.IsAllowed(path)
		assert.True(t, allowed, "Path should be allowed: %s", path)
	}
}

func TestPathValidator_RejectTraversal(t *testing.T) {
	allowedDirs := []string{"/tmp/workspace"}
	validator := NewPathValidator(allowedDirs)

	testCases := []string{
		"/tmp/workspace/../etc/passwd",
		"/tmp/workspace/subdir/../../outside.txt",
		"../etc/passwd",
	}

	for _, path := range testCases {
		err := validator.ValidatePath(path)
		require.Error(t, err, "Path should be rejected: %s", path)
		assert.Contains(t, err.Error(), "path traversal", "Error should mention traversal for: %s", path)

		allowed := validator.IsAllowed(path)
		assert.False(t, allowed, "Path should be rejected: %s", path)
	}
}

func TestPathValidator_RejectAbsoluteOutsideWorkspace(t *testing.T) {
	allowedDirs := []string{"/tmp/workspace"}
	validator := NewPathValidator(allowedDirs)

	testCases := []string{
		"/etc/passwd",
		"/home/user/file.txt",
		"/var/log/syslog",
	}

	for _, path := range testCases {
		err := validator.ValidatePath(path)
		require.Error(t, err, "Path should be rejected: %s", path)
		assert.Contains(t, err.Error(), "outside allowed workspace", "Error should mention workspace for: %s", path)

		allowed := validator.IsAllowed(path)
		assert.False(t, allowed, "Path should be rejected: %s", path)
	}
}

func TestPathValidator_CanonicalizePath(t *testing.T) {
	// Get current working directory for relative path test
	cwd, err := filepath.Abs(".")
	require.NoError(t, err)

	allowedDirs := []string{cwd}
	validator := NewPathValidator(allowedDirs)

	// Relative paths should be canonicalized to absolute
	err = validator.ValidatePath("./test.txt")
	assert.NoError(t, err, "Relative path within workspace should be allowed")

	err = validator.ValidatePath("test.txt")
	assert.NoError(t, err, "Simple filename within workspace should be allowed")
}

func TestPathValidator_MultipleAllowedDirs(t *testing.T) {
	allowedDirs := []string{"/tmp/workspace1", "/tmp/workspace2", "/home/user/projects"}
	validator := NewPathValidator(allowedDirs)

	// Paths in different allowed directories should all be allowed
	testCases := []string{
		"/tmp/workspace1/file.txt",
		"/tmp/workspace2/file.txt",
		"/home/user/projects/file.txt",
	}

	for _, path := range testCases {
		err := validator.ValidatePath(path)
		assert.NoError(t, err, "Path should be allowed: %s", path)
	}

	// Path outside all allowed directories should be rejected
	err := validator.ValidatePath("/tmp/workspace3/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed workspace")
}

func TestPathValidator_EmptyAllowedDirs(t *testing.T) {
	validator := NewPathValidator([]string{})

	// All paths should be rejected if no allowed directories
	err := validator.ValidatePath("/tmp/test.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed workspace")
}

func TestPathValidator_CleanPath(t *testing.T) {
	allowedDirs := []string{"/tmp/workspace"}
	validator := NewPathValidator(allowedDirs)

	// Paths with redundant separators should be cleaned and allowed
	testCases := []string{
		"/tmp/workspace//file.txt",
		"/tmp/workspace/./file.txt",
		"/tmp/workspace/subdir/../file.txt", // Will be rejected due to ".." detection
	}

	err := validator.ValidatePath(testCases[0])
	assert.NoError(t, err, "Path with double slash should be cleaned and allowed")

	err = validator.ValidatePath(testCases[1])
	assert.NoError(t, err, "Path with ./ should be cleaned and allowed")

	err = validator.ValidatePath(testCases[2])
	require.Error(t, err, "Path with .. should be rejected")
}
