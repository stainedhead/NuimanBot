// Package memoryv2 implements the self-organizing memory v2 domain entities.
package memoryv2

import "fmt"

// CellType represents the type of knowledge in a memory cell.
type CellType int

const (
	// CellTypeFact represents objective information.
	CellTypeFact CellType = iota
	// CellTypeDecision represents a choice that was made.
	CellTypeDecision
	// CellTypeTask represents something to do or track.
	CellTypeTask
	// CellTypePreference represents user's preference or style.
	CellTypePreference
	// CellTypePlan represents future intention or strategy.
	CellTypePlan
	// CellTypeRisk represents potential issue or concern.
	CellTypeRisk
)

var cellTypeStrings = [...]string{
	"fact",
	"decision",
	"task",
	"preference",
	"plan",
	"risk",
}

// String returns the string representation of the CellType.
func (c CellType) String() string {
	if !c.IsValid() {
		return "unknown"
	}
	return cellTypeStrings[c]
}

// IsValid checks if the CellType value is valid.
func (c CellType) IsValid() bool {
	return c >= CellTypeFact && c <= CellTypeRisk
}

// ParseCellType parses a string to CellType.
func ParseCellType(s string) (CellType, error) {
	switch s {
	case "fact":
		return CellTypeFact, nil
	case "decision":
		return CellTypeDecision, nil
	case "task":
		return CellTypeTask, nil
	case "preference":
		return CellTypePreference, nil
	case "plan":
		return CellTypePlan, nil
	case "risk":
		return CellTypeRisk, nil
	default:
		return 0, fmt.Errorf("invalid cell type: %q", s)
	}
}

// AllCellTypes returns a slice of all valid CellType values.
func AllCellTypes() []CellType {
	return []CellType{
		CellTypeFact,
		CellTypeDecision,
		CellTypeTask,
		CellTypePreference,
		CellTypePlan,
		CellTypeRisk,
	}
}
