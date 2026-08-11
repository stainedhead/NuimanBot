package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

// TestRunEventsJS_ServedAndWiredIntoDetailPages is FR-R10's structural test:
// it cannot exercise a live WebSocket round-trip through a browser (no
// browser in this test environment — see implementation-notes.md's
// Deviations from Plan for the manual-verification note this leaves), but
// it proves every piece a browser needs is actually present and correctly
// linked: the script is served at the expected static path, contains a
// real WebSocket client and the three documented event-type strings, and
// each of the three detail pages this finding targets (Job/Chore/Run)
// includes the script tag plus the DOM anchors (IDs/data attributes) the
// script depends on to know what to update.
func TestRunEventsJS_ServedAndWiredIntoDetailPages(t *testing.T) {
	server := NewServer(":0")

	t.Run("script is served with real WebSocket client code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/run-events.js", nil)
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected /static/run-events.js to be served, got %d", w.Code)
		}
		body := w.Body.String()
		for _, want := range []string{"new WebSocket(", "run_status", "run_log", "notification_badge"} {
			if !strings.Contains(body, want) {
				t.Errorf("expected run-events.js to contain %q, it did not", want)
			}
		}
	})

	t.Run("Job detail page wires the script and DOM anchors", func(t *testing.T) {
		server.SetJobsService(NewMockJobsService())
		server.jobsService.(*MockJobsService).jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "alice", Title: "Weekly report"}

		req := httptest.NewRequest(http.MethodGet, "/admin/jobs/job-1", nil)
		req.AddCookie(sessionCookieFor(server, "alice", "user"))
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		assertContainsAll(t, body, []string{
			`<script src="/static/run-events.js" defer></script>`,
			`data-source-type="job"`,
			`data-source-id="job-1"`,
			`id="job-status"`,
		})
	})

	t.Run("Chore detail page wires the script and DOM anchors", func(t *testing.T) {
		server.SetChoresService(NewMockChoresService())
		server.choresService.(*MockChoresService).chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice", Title: "Nightly cleanup"}

		req := httptest.NewRequest(http.MethodGet, "/admin/chores/chore-1", nil)
		req.AddCookie(sessionCookieFor(server, "alice", "user"))
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		assertContainsAll(t, body, []string{
			`<script src="/static/run-events.js" defer></script>`,
			`data-source-type="chore"`,
			`data-source-id="chore-1"`,
			`id="chore-status"`,
		})
	})

	t.Run("Run detail page wires the script and DOM anchors", func(t *testing.T) {
		server.SetHistoryService(NewMockHistoryService())
		server.historyService.(*MockHistoryService).runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusRunning}

		req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
		req.AddCookie(sessionCookieFor(server, "alice", "user"))
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		assertContainsAll(t, body, []string{
			`<script src="/static/run-events.js" defer></script>`,
			`data-run-id="run-1"`,
			`id="run-status"`,
			`id="run-log"`,
		})
	})
}

func assertContainsAll(t *testing.T, body string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("expected rendered page to contain %q, it did not; body: %s", want, body)
		}
	}
}

// TestRunEventsJS_ServerPublishedSourceTypeMatchesPageDataAttribute closes
// the gap a structural-only test (above) can't: it proves the exact wire
// value notifyingRunRepository.SaveRun actually publishes for a Job-sourced
// Run's run_status event (domain.SourceTypeJob, JSON-marshaled) is the
// identical literal string job_detail.html renders into data-source-type —
// not just that both independently look like "job" to a human reader. If
// domain.SourceType's string value or the JSON field name ever drifts from
// what run-events.js's matchesThisPage() compares against
// (event.sourceType === body.dataset.sourceType), this test — not a
// browser — is what catches it.
func TestRunEventsJS_ServerPublishedSourceTypeMatchesPageDataAttribute(t *testing.T) {
	repo, hub := newNotifyingRunRepoTestFixture(t)
	_, recv := registerCapturingClient(hub, "alice")

	run := &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusRunning}
	if err := repo.SaveRun(t.Context(), run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	var published string
	select {
	case msg := <-recv:
		published = string(msg)
	default:
		t.Fatal("expected SaveRun to publish a run_status event")
	}

	// job_detail.html renders data-source-type="job" data-source-id="job-1"
	// (see TestRunEventsJS_ServedAndWiredIntoDetailPages) — these are the
	// exact substrings run-events.js's matchesThisPage() needs to find in
	// the published event for a Job's live status update to ever fire.
	for _, want := range []string{`"sourceType":"job"`, `"sourceId":"job-1"`} {
		if !strings.Contains(published, want) {
			t.Fatalf("published run_status event %q does not contain %q — job_detail.html's data-source-type/data-source-id matching would never fire", published, want)
		}
	}
}
