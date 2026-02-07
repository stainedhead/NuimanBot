package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AssertNoError is a test helper that fails the test if err is not nil
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	assert.NoError(t, err, msgAndArgs...)
}

// AssertError is a test helper that fails the test if err is nil
func AssertError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Error(t, err, msgAndArgs...)
}

// AssertContains is a test helper that fails the test if s does not contain substring
func AssertContains(t *testing.T, s, substring string, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Contains(t, s, substring, msgAndArgs...)
}

// AssertEqual is a test helper that fails the test if expected != actual
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	assert.Equal(t, expected, actual, msgAndArgs...)
}
