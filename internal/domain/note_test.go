package domain

import (
	"strings"
	"testing"
)

func TestNote_Validate(t *testing.T) {
	tests := []struct {
		name    string
		note    *Note
		wantErr bool
	}{
		{
			name: "valid note",
			note: &Note{
				UserID:  "user-1",
				Title:   "My Note",
				Content: "Some content here.",
			},
			wantErr: false,
		},
		{
			name: "missing user ID",
			note: &Note{
				Title:   "My Note",
				Content: "Some content.",
			},
			wantErr: true,
		},
		{
			name: "missing title",
			note: &Note{
				UserID:  "user-1",
				Content: "Some content.",
			},
			wantErr: true,
		},
		{
			name: "missing content",
			note: &Note{
				UserID: "user-1",
				Title:  "My Note",
			},
			wantErr: true,
		},
		{
			name: "content too long",
			note: &Note{
				UserID:  "user-1",
				Title:   "My Note",
				Content: strings.Repeat("a", 100001),
			},
			wantErr: true,
		},
		{
			name: "content at max length",
			note: &Note{
				UserID:  "user-1",
				Title:   "My Note",
				Content: strings.Repeat("a", 100000),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.note.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
