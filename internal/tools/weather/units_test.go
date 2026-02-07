package weather

import (
	"testing"
)

func TestGetUnitsSymbol(t *testing.T) {
	w := NewWeather("test-key", 10)

	tests := []struct {
		units      string
		wantSymbol string
	}{
		{"metric", "°C"},
		{"imperial", "°F"},
		{"standard", "K"},
		{"invalid", ""},
		{"", ""},
		{"METRIC", ""},
		{"Celsius", ""},
	}

	for _, tt := range tests {
		t.Run(tt.units, func(t *testing.T) {
			symbol := w.getUnitsSymbol(tt.units)
			if symbol != tt.wantSymbol {
				t.Errorf("getUnitsSymbol(%s) = %s, want %s", tt.units, symbol, tt.wantSymbol)
			}
		})
	}
}
