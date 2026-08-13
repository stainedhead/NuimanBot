package cli

import (
	"context"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/settings"
)

// fakePoolController is a minimal settings.PoolController test double.
type fakePoolController struct {
	concurrency int
}

func (f *fakePoolController) SetConcurrency(n int) { f.concurrency = n }
func (f *fakePoolController) Concurrency() int     { return f.concurrency }

// fakeSkillsLister is a minimal settings.SkillsLister test double.
type fakeSkillsLister struct {
	names []string
}

func (f *fakeSkillsLister) SkillNames() []string { return f.names }

func newTestSettingsHandler() (*SettingsCommandHandler, *fakePoolController) {
	pool := &fakePoolController{concurrency: 4}
	skills := &fakeSkillsLister{names: []string{"notes", "websearch"}}
	svc := settings.NewService(pool, skills, settings.RetentionDefaults{
		ChatDays:    30,
		ProjectDays: 0, // Never
		HistoryDays: 14,
	})
	return NewSettingsCommandHandler(svc), pool
}

var (
	adminUser    = &domain.User{ID: "a1", Username: "admin", Role: domain.RoleAdmin}
	regularUser  = &domain.User{ID: "u1", Username: "bob", Role: domain.RoleUser}
	guestUserPtr *domain.User // nil, exercises the nil-currentUser defensive path
)

// TestSettingsShow_DisplaysSystemWideRetentionReadOnly verifies "/settings
// show" displays the system-wide retention defaults (not a fictional
// per-user value — see spec.md FR-039's corrected text) for any
// authenticated user, including "Never" for a zero-day value.
func TestSettingsShow_DisplaysSystemWideRetentionReadOnly(t *testing.T) {
	h, _ := newTestSettingsHandler()

	out, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings show")
	if err != nil {
		t.Fatalf("HandleSettingsCommand: %v", err)
	}
	if !strings.Contains(out, "Chat retention:    30 days") {
		t.Errorf("expected chat retention 30 days, got:\n%s", out)
	}
	if !strings.Contains(out, "Project retention: Never") {
		t.Errorf("expected project retention Never, got:\n%s", out)
	}
	if !strings.Contains(out, "History retention: 14 days") {
		t.Errorf("expected history retention 14 days, got:\n%s", out)
	}
}

// TestSettingsShowSystem_AdminOnly verifies "/settings show --system" is
// rejected for a non-admin and succeeds for an admin, per FR-041.
func TestSettingsShowSystem_AdminOnly(t *testing.T) {
	h, _ := newTestSettingsHandler()

	if _, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings show --system"); err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions for non-admin, got %v", err)
	}
	if _, err := h.HandleSettingsCommand(context.Background(), guestUserPtr, "", "/settings show --system"); err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions for nil user, got %v", err)
	}

	out, err := h.HandleSettingsCommand(context.Background(), adminUser, "admin", "/settings show --system")
	if err != nil {
		t.Fatalf("expected admin to succeed, got %v", err)
	}
	if !strings.Contains(out, "Worker pool size:  4") {
		t.Errorf("expected worker pool size 4, got:\n%s", out)
	}
	if !strings.Contains(out, "Skills registered: 2") {
		t.Errorf("expected 2 skills registered, got:\n%s", out)
	}
	if !strings.Contains(out, "not yet implemented") || !strings.Contains(out, "FR-R11") {
		t.Errorf("expected network mode to reference the FR-043/FR-R11 deferral, got:\n%s", out)
	}
}

// TestSettingsSetWorkerPoolSize_AdminOnlyAndValidated verifies FR-042: real
// worker-pool-size updates, admin-gated, with validation.
func TestSettingsSetWorkerPoolSize_AdminOnlyAndValidated(t *testing.T) {
	h, pool := newTestSettingsHandler()

	if _, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings set worker-pool-size 10"); err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions for non-admin, got %v", err)
	}
	if pool.concurrency != 4 {
		t.Errorf("non-admin attempt must not change pool concurrency, got %d", pool.concurrency)
	}

	out, err := h.HandleSettingsCommand(context.Background(), adminUser, "admin", "/settings set worker-pool-size 10")
	if err != nil {
		t.Fatalf("expected admin to succeed, got %v", err)
	}
	if pool.concurrency != 10 {
		t.Errorf("expected pool concurrency updated to 10, got %d", pool.concurrency)
	}
	if !strings.Contains(out, "10") {
		t.Errorf("expected confirmation to mention the new size, got %q", out)
	}

	if _, err := h.HandleSettingsCommand(context.Background(), adminUser, "admin", "/settings set worker-pool-size notanumber"); err == nil {
		t.Error("expected an error for a non-numeric worker-pool-size")
	}
	if _, err := h.HandleSettingsCommand(context.Background(), adminUser, "admin", "/settings set worker-pool-size 0"); err == nil {
		t.Error("expected an error for a non-positive worker-pool-size")
	}
	if _, err := h.HandleSettingsCommand(context.Background(), adminUser, "admin", "/settings set worker-pool-size 99999"); err == nil {
		t.Error("expected an error for a worker-pool-size exceeding domain.MaxWorkerPoolSize")
	}
}

// TestSettingsSetRetention_Deferred verifies FR-040's deferral: invoking it
// returns a clear not-yet-implemented message, not silent success or a
// panic, and does not require admin (matching how /settings show is
// available to any authenticated user, even though the "set" action itself
// does nothing yet).
func TestSettingsSetRetention_Deferred(t *testing.T) {
	h, _ := newTestSettingsHandler()

	out, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings set retention chat never")
	if err != nil {
		t.Fatalf("expected no error (a deferred stub, not a hard failure), got %v", err)
	}
	if !strings.Contains(out, "not yet implemented") {
		t.Errorf("expected a clear not-yet-implemented message, got %q", out)
	}
}

// TestSettingsSetNetworkMode_DeferredAndAdminGated verifies FR-043's
// deferral: admin-gated (matching its "admin-only" spec text even though
// unimplemented), and returns a clear message referencing the FR-R11 root
// blocker rather than silently doing nothing.
func TestSettingsSetNetworkMode_DeferredAndAdminGated(t *testing.T) {
	h, _ := newTestSettingsHandler()

	if _, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings set network-mode remote"); err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions for non-admin, got %v", err)
	}

	out, err := h.HandleSettingsCommand(context.Background(), adminUser, "admin", "/settings set network-mode remote")
	if err != nil {
		t.Fatalf("expected no error (a deferred stub, not a hard failure), got %v", err)
	}
	if !strings.Contains(out, "not yet implemented") || !strings.Contains(out, "FR-R11") {
		t.Errorf("expected a message referencing the FR-043/FR-R11 deferral, got %q", out)
	}
}

// TestSettingsBareCommand_ShowsHelp verifies bare "/settings" shows help
// rather than an error.
func TestSettingsBareCommand_ShowsHelp(t *testing.T) {
	h, _ := newTestSettingsHandler()

	out, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings")
	if err != nil {
		t.Fatalf("HandleSettingsCommand: %v", err)
	}
	if !strings.Contains(out, "Settings commands:") {
		t.Errorf("expected help output for bare /settings, got:\n%s", out)
	}
}

// TestSettingsUnknownSubcommand_ShowsHelp verifies an unrecognized
// subcommand falls back to help rather than a cryptic error.
func TestSettingsUnknownSubcommand_ShowsHelp(t *testing.T) {
	h, _ := newTestSettingsHandler()

	out, err := h.HandleSettingsCommand(context.Background(), regularUser, "bob", "/settings frobnicate")
	if err != nil {
		t.Fatalf("HandleSettingsCommand: %v", err)
	}
	if !strings.Contains(out, "Settings commands:") {
		t.Errorf("expected help output for unrecognized subcommand, got:\n%s", out)
	}
}

// TestSettingsHandle_SatisfiesEnvCommandHandler verifies the exported Handle
// method (satisfying cli.EnvCommandHandler) delegates correctly.
func TestSettingsHandle_SatisfiesEnvCommandHandler(t *testing.T) {
	h, _ := newTestSettingsHandler()
	var _ EnvCommandHandler = h

	out, err := h.Handle(context.Background(), regularUser, "bob", "/settings show")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(out, "Chat retention:") {
		t.Errorf("expected retention output via Handle, got:\n%s", out)
	}
}
