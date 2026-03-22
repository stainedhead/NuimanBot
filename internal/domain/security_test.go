package domain

import (
	"testing"
)

func TestNewSecureString(t *testing.T) {
	val := []byte("secret")
	ss := NewSecureString(val)
	if ss.Value() != "secret" {
		t.Errorf("NewSecureString() Value() = %q, want %q", ss.Value(), "secret")
	}
}

func TestNewSecureStringFromString(t *testing.T) {
	ss := NewSecureStringFromString("my-api-key")
	if ss.Value() != "my-api-key" {
		t.Errorf("NewSecureStringFromString() Value() = %q, want %q", ss.Value(), "my-api-key")
	}
}

func TestSecureString_Value(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"non-empty", "token-12345"},
		{"empty string", ""},
		{"unicode", "パスワード"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := NewSecureStringFromString(tt.input)
			if got := ss.Value(); got != tt.input {
				t.Errorf("Value() = %q, want %q", got, tt.input)
			}
		})
	}
}

func TestSecureString_Zero(t *testing.T) {
	ss := NewSecureStringFromString("secret-data")

	// Before zeroing, value should be available
	if ss.Value() == "" {
		t.Error("Value should be non-empty before zeroing")
	}

	ss.Zero()

	// After zeroing, value should be empty
	if ss.Value() != "" {
		t.Errorf("After Zero(), Value() = %q, want empty string", ss.Value())
	}

	// Second Zero() should be safe (nil check)
	ss.Zero()
	if ss.Value() != "" {
		t.Error("Second Zero() should be safe")
	}
}

func TestSecureString_ZeroNilValue(t *testing.T) {
	var ss SecureString
	// Should not panic
	ss.Zero()
}
