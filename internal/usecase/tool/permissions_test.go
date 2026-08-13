package tool_test

import (
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/tools/buzzsend"
	"nuimanbot/internal/tools/calculator"
	"nuimanbot/internal/tools/datetime"
	"nuimanbot/internal/tools/notes"
	"nuimanbot/internal/tools/weather"
	"nuimanbot/internal/tools/websearch"
	. "nuimanbot/internal/usecase/tool"
	"nuimanbot/internal/usecase/tool/coding_agent"
	"nuimanbot/internal/usecase/tool/common"
	"nuimanbot/internal/usecase/tool/doc_summarize"
	"nuimanbot/internal/usecase/tool/github"
	"nuimanbot/internal/usecase/tool/repo_search"
	"nuimanbot/internal/usecase/tool/summarize"
)

// newProductionLikeRegistry builds a ToolRegistry populated with the same
// tools cmd/nuimanbot's registerBuiltInTools / registerDeveloperProductivityTools
// register in production, plus buzzsend — registered separately by
// cmd/nuimanbot/acp.go's runACP (ACP-only, not part of
// registerBuiltInTools) but included here anyway so this guard actually
// covers it: its absence from ToolPermissions previously went undetected by
// this same guard and silently hid the tool from every Guest-role user (the
// bug this entry now catches). Dependencies that Name() never touches
// (executors, LLM services, HTTP clients, notes repositories) are passed as
// nil/zero values — safe here since this registry exists only to exercise
// Name() and Register(), never Execute().
//
// weather is registered unconditionally here (production gates it on
// OPENWEATHERMAP_API_KEY being set) so this guard checks its permission
// entry regardless of what's set in the environment running the test.
//
// MCP tools are dynamically registered under the "mcp:<server>:<tool>"
// namespace and are intentionally NOT included here: their RBAC treatment is
// Phase 6 / FR-023's responsibility (trust-based classification resolved at
// bridge time, not a static ToolPermissions entry), not this Phase 3 guard's.
func newProductionLikeRegistry(t *testing.T) ToolRegistry {
	t.Helper()

	registry := NewInMemoryRegistry()

	tools := []domain.Tool{
		calculator.NewCalculator(),
		datetime.NewDateTime(),
		weather.NewWeather("test-api-key", 10),
		websearch.NewWebSearch(10),
		notes.NewNotes(nil),
		github.NewGitHubSkill(domain.ToolConfig{}, nil, common.NewRateLimiter(), common.NewOutputSanitizer()),
		repo_search.NewRepoSearchSkill(domain.ToolConfig{}, nil, common.NewPathValidator(nil), common.NewOutputSanitizer()),
		doc_summarize.NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil),
		summarize.NewSummarizeSkill(domain.ToolConfig{}, nil, nil, nil),
		coding_agent.NewCodingAgentSkill(domain.ToolConfig{}, nil, common.NewPathValidator(nil)),
		buzzsend.New(),
	}

	for _, tl := range tools {
		if err := registry.Register(tl); err != nil {
			t.Fatalf("failed to register %q in test registry: %v", tl.Name(), err)
		}
	}

	return registry
}

// TestToolPermissions_EveryRegisteredToolHasExplicitEntry is the CI guard
// required by tasks.md P3.3: it fails the build if a tool registered by the
// production wiring has no deliberate entry in ToolPermissions, forcing a
// conscious RBAC decision for every new tool rather than a silent
// fall-through to DefaultToolPermission.
func TestToolPermissions_EveryRegisteredToolHasExplicitEntry(t *testing.T) {
	registry := newProductionLikeRegistry(t)

	for _, tl := range registry.List() {
		name := tl.Name()
		if _, ok := ToolPermissions[name]; !ok {
			t.Errorf("tool %q is registered but has no explicit ToolPermissions entry in "+
				"internal/usecase/tool/permissions.go — add one (see that file's doc "+
				"comment for how to classify read-only vs. side-effecting tools)", name)
		}
	}
}

// TestToolPermissions_ExpectedRoles locks in the exact role assignments
// required by specs/260802-improve-nuimanbot-security/tasks.md P3.1.
func TestToolPermissions_ExpectedRoles(t *testing.T) {
	expected := map[string]domain.Role{
		"calculator":        domain.RoleGuest,
		"datetime":          domain.RoleGuest,
		"weather":           domain.RoleUser,
		"websearch":         domain.RoleUser,
		"notes":             domain.RoleUser,
		"repo_search":       domain.RoleUser,
		"doc_summarize":     domain.RoleUser,
		"summarize":         domain.RoleUser,
		"github":            domain.RoleAdmin,
		"coding_agent":      domain.RoleAdmin,
		"buzz_send_message": domain.RoleGuest,
	}

	for name, wantRole := range expected {
		gotRole, ok := ToolPermissions[name]
		if !ok {
			t.Errorf("expected explicit ToolPermissions entry for %q, found none", name)
			continue
		}
		if gotRole != wantRole {
			t.Errorf("ToolPermissions[%q] = %q, want %q", name, gotRole, wantRole)
		}
	}
}
