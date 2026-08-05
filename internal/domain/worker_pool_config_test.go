package domain

import (
	"errors"
	"testing"
)

func TestWorkerPoolConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		n       int
		wantErr bool
	}{
		{"positive", 3, false},
		{"one", 1, false},
		{"zero", 0, true},
		{"negative", -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := WorkerPoolConfig{MaxConcurrentWorkers: tc.n}.Validate()
			if tc.wantErr && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput for n=%d, got %v", tc.n, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for n=%d, got %v", tc.n, err)
			}
		})
	}
}
