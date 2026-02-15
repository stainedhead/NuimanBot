package memoryv2

import (
	"errors"
	"testing"
)

func TestErrors_AreDistinct(t *testing.T) {
	errs := []error{ErrNotFound, ErrAlreadyExists, ErrInvalidInput}

	for i, err1 := range errs {
		for j, err2 := range errs {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("Expected %v and %v to be distinct", err1, err2)
			}
		}
	}
}

func TestErrors_Messages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrNotFound", ErrNotFound, "not found"},
		{"ErrAlreadyExists", ErrAlreadyExists, "already exists"},
		{"ErrInvalidInput", ErrInvalidInput, "invalid input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
