package jobs

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// TestService_DoesNotImportOSOrFsguard is the FR-R5 regression test: this
// usecase package must depend only on domain.ConfinedFileStore for
// confined filesystem I/O, never directly on "os" or
// internal/infrastructure/fsguard (AGENTS.md's Clean Architecture
// dependency rule — inner layers define interfaces, outer layers implement
// them). Parses service.go's own import declarations rather than grepping,
// so a reference inside a comment or string literal can't produce a false
// positive.
func TestService_DoesNotImportOSOrFsguard(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "service.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse service.go: %v", err)
	}

	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("failed to unquote import path %q: %v", imp.Path.Value, err)
		}
		if path == "os" {
			t.Error(`service.go must not import "os" directly — use domain.ConfinedFileStore (FR-R5)`)
		}
		if path == "nuimanbot/internal/infrastructure/fsguard" {
			t.Error(`service.go must not import internal/infrastructure/fsguard directly — use domain.ConfinedFileStore (FR-R5)`)
		}
	}
}
