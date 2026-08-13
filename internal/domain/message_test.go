package domain_test

import (
	"testing"

	"nuimanbot/internal/domain"
)

func TestPlatformConstants(t *testing.T) {
	tests := []struct {
		name     string
		platform domain.Platform
		want     string
	}{
		{"telegram", domain.PlatformTelegram, "telegram"},
		{"slack", domain.PlatformSlack, "slack"},
		{"cli", domain.PlatformCLI, "cli"},
		{"buzz", domain.PlatformBuzz, "buzz"},
		{"acp", domain.PlatformACP, "acp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.platform) != tt.want {
				t.Errorf("got %q, want %q", tt.platform, tt.want)
			}
		})
	}
}
