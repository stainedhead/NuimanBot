package factory

import (
	"strings"
	"testing"
)

// TestValidateStorePrefix tests the validateStorePrefix helper directly.
// This is a white-box test in the factory package to access the unexported function.
func TestValidateStorePrefix(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr bool
	}{
		// Empty string is always accepted — factory substitutes "nuiman".
		{name: "empty accepts default", prefix: "", wantErr: false},

		// Valid prefixes.
		{name: "nuiman (default)", prefix: "nuiman", wantErr: false},
		{name: "valid hyphenated", prefix: "valid-prefix-123", wantErr: false},
		{name: "two chars minimum", prefix: "ab", wantErr: false},
		{name: "31 chars maximum", prefix: "a123456789012345678901234567890", wantErr: false},
		{name: "starts with digit", prefix: "1bot", wantErr: false},

		// Invalid prefixes.
		{name: "spaces and special chars", prefix: "My Bot!", wantErr: true},
		{name: "single char (too short)", prefix: "a", wantErr: true},
		{name: "32 chars (too long)", prefix: "a1234567890123456789012345678901", wantErr: true},
		{name: "uppercase letters", prefix: "MyBot", wantErr: true},
		{name: "leading hyphen", prefix: "-bot", wantErr: true},
		{name: "contains space", prefix: "my bot", wantErr: true},
		{name: "contains underscore", prefix: "my_bot", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStorePrefix(tt.prefix)
			if tt.wantErr && err == nil {
				t.Errorf("validateStorePrefix(%q): expected error, got nil", tt.prefix)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateStorePrefix(%q): expected no error, got %v", tt.prefix, err)
			}
			// When invalid, the error message must contain "store_prefix".
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "store_prefix") {
					t.Errorf("validateStorePrefix(%q): error %q does not contain %q", tt.prefix, err.Error(), "store_prefix")
				}
			}
		})
	}
}
