package nostr

import (
	"testing"
	"time"
)

func TestParseEventFrame(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantOK    bool
		wantErr   bool
		wantKind  int
		wantEvent bool
	}{
		{
			name:    "invalid JSON",
			data:    `not json`,
			wantErr: true,
		},
		{
			name:   "frame too short is ignored",
			data:   `["EOSE","sub-id"]`,
			wantOK: false,
		},
		{
			name:    "non-string message type",
			data:    `[123,"sub-id",{}]`,
			wantErr: true,
		},
		{
			name:   "non-EVENT message type is ignored",
			data:   `["OK","event-id",true,""]`,
			wantOK: false,
		},
		{
			name:    "malformed event payload",
			data:    `["EVENT","sub-id","not-an-object"]`,
			wantErr: true,
		},
		{
			name:      "valid EVENT frame",
			data:      `["EVENT","sub-id",{"id":"abc","pubkey":"pk","created_at":100,"kind":42,"tags":[["e","x"]],"content":"hi","sig":"sig"}]`,
			wantOK:    true,
			wantKind:  42,
			wantEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok, err := parseEventFrame([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseEventFrame() error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantOK {
				t.Fatalf("parseEventFrame() ok = %v, wantOK %v", ok, tt.wantOK)
			}
			if tt.wantEvent && event.Kind != tt.wantKind {
				t.Errorf("event.Kind = %d, want %d", event.Kind, tt.wantKind)
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		name    string
		current time.Duration
		max     time.Duration
		want    time.Duration
	}{
		{name: "doubles below max", current: time.Second, max: 100 * time.Second, want: 2 * time.Second},
		{name: "caps at max", current: 60 * time.Second, max: 100 * time.Second, want: 100 * time.Second},
		{name: "already at max", current: 100 * time.Second, max: 100 * time.Second, want: 100 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextBackoff(tt.current, tt.max)
			if got != tt.want {
				t.Errorf("nextBackoff(%v, %v) = %v, want %v", tt.current, tt.max, got, tt.want)
			}
		})
	}
}
