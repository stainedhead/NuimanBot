package memoryv2

import (
	"testing"
)

func TestCellType_String(t *testing.T) {
	tests := []struct {
		name     string
		cellType CellType
		want     string
	}{
		{"fact", CellTypeFact, "fact"},
		{"decision", CellTypeDecision, "decision"},
		{"task", CellTypeTask, "task"},
		{"preference", CellTypePreference, "preference"},
		{"plan", CellTypePlan, "plan"},
		{"risk", CellTypeRisk, "risk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cellType.String(); got != tt.want {
				t.Errorf("CellType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCellType_String_Invalid(t *testing.T) {
	invalid := CellType(99)
	got := invalid.String()
	if got != "unknown" {
		t.Errorf("CellType.String() for invalid = %v, want 'unknown'", got)
	}
}

func TestCellType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		cellType CellType
		want     bool
	}{
		{"fact is valid", CellTypeFact, true},
		{"decision is valid", CellTypeDecision, true},
		{"task is valid", CellTypeTask, true},
		{"preference is valid", CellTypePreference, true},
		{"plan is valid", CellTypePlan, true},
		{"risk is valid", CellTypeRisk, true},
		{"negative is invalid", CellType(-1), false},
		{"too large is invalid", CellType(99), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cellType.IsValid(); got != tt.want {
				t.Errorf("CellType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCellType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    CellType
		wantErr bool
	}{
		{"parse fact", "fact", CellTypeFact, false},
		{"parse decision", "decision", CellTypeDecision, false},
		{"parse task", "task", CellTypeTask, false},
		{"parse preference", "preference", CellTypePreference, false},
		{"parse plan", "plan", CellTypePlan, false},
		{"parse risk", "risk", CellTypeRisk, false},
		{"empty string", "", 0, true},
		{"invalid type", "invalid", 0, true},
		{"uppercase", "FACT", 0, true},
		{"mixed case", "Fact", 0, true},
		{"with spaces", " fact ", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCellType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCellType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseCellType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCellType_RoundTrip(t *testing.T) {
	// Test that parsing the string representation returns the original
	for ct := CellTypeFact; ct <= CellTypeRisk; ct++ {
		str := ct.String()
		parsed, err := ParseCellType(str)
		if err != nil {
			t.Errorf("Failed to parse %q: %v", str, err)
			continue
		}
		if parsed != ct {
			t.Errorf("Round trip failed for %v: got %v", ct, parsed)
		}
	}
}

func TestAllCellTypes(t *testing.T) {
	all := AllCellTypes()
	if len(all) != 6 {
		t.Errorf("AllCellTypes() returned %d types, want 6", len(all))
	}

	expected := []CellType{
		CellTypeFact,
		CellTypeDecision,
		CellTypeTask,
		CellTypePreference,
		CellTypePlan,
		CellTypeRisk,
	}

	for i, ct := range expected {
		if all[i] != ct {
			t.Errorf("AllCellTypes()[%d] = %v, want %v", i, all[i], ct)
		}
	}
}
