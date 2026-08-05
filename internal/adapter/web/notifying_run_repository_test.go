package web

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

func newNotifyingRunRepoTestFixture(t *testing.T) (domain.RunRepository, *Hub) {
	t.Helper()
	base := storage.NewFileRunRepository(t.TempDir())
	hub := NewHub()
	return NewNotifyingRunRepository(base, hub), hub
}

// registerCapturingClient registers a synchronous, unbuffered-drain client
// directly against hub (bypassing the WebSocket transport) so tests can
// assert exactly what Hub.Publish was called with, without a real
// connection.
func registerCapturingClient(hub *Hub, ownerUserID string) (*wsClient, chan []byte) {
	client := &wsClient{ownerUserID: ownerUserID, send: make(chan []byte, 8)}
	hub.register(client)
	return client, client.send
}

func TestNotifyingRunRepository_SaveRun_PublishesRunStatus(t *testing.T) {
	repo, hub := newNotifyingRunRepoTestFixture(t)
	_, recv := registerCapturingClient(hub, "alice")

	run := &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusRunning}
	if err := repo.SaveRun(context.Background(), run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	select {
	case msg := <-recv:
		s := string(msg)
		for _, want := range []string{`"type":"run_status"`, `"runId":"run-1"`, `"status":"running"`} {
			if !strings.Contains(s, want) {
				t.Errorf("expected published event to contain %q, got %s", want, s)
			}
		}
	default:
		t.Fatal("expected SaveRun to publish a run_status event")
	}
}

func TestNotifyingRunRepository_AppendLog_PublishesRunLog(t *testing.T) {
	repo, hub := newNotifyingRunRepoTestFixture(t)
	ctx := context.Background()
	run := &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusRunning}
	if err := repo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	_, recv := registerCapturingClient(hub, "alice")

	if err := repo.AppendLog(ctx, "alice", "run-1", "hello\n"); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}

	select {
	case msg := <-recv:
		s := string(msg)
		if !strings.Contains(s, `"type":"run_log"`) || !strings.Contains(s, `"logChunk":"hello`) {
			t.Fatalf("unexpected event payload: %s", s)
		}
	default:
		t.Fatal("expected AppendLog to publish a run_log event")
	}
}

func TestNotifyingRunRepository_MarkNotified_PublishesRefreshedBadgeCount(t *testing.T) {
	repo, hub := newNotifyingRunRepoTestFixture(t)
	ctx := context.Background()
	run := &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted}
	if err := repo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	_, recv := registerCapturingClient(hub, "alice")

	if err := repo.MarkNotified(ctx, "alice", "run-1"); err != nil {
		t.Fatalf("MarkNotified: %v", err)
	}

	select {
	case msg := <-recv:
		s := string(msg)
		if !strings.Contains(s, `"type":"notification_badge"`) || !strings.Contains(s, `"unnotifiedCount":0`) {
			t.Fatalf("unexpected event payload: %s", s)
		}
	default:
		t.Fatal("expected MarkNotified to publish a notification_badge event")
	}
}

func TestNotifyingRunRepository_ErrorFromWrappedRepo_SkipsPublish(t *testing.T) {
	repo, hub := newNotifyingRunRepoTestFixture(t)
	_, recv := registerCapturingClient(hub, "alice")

	// AppendLog against a run that was never saved returns ErrNotFound —
	// the decorator must propagate that error and must not publish.
	err := repo.AppendLog(context.Background(), "alice", "does-not-exist", "chunk")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	select {
	case msg := <-recv:
		t.Fatalf("expected no event to be published on error, got %s", msg)
	default:
	}
}

func TestNotifyingRunRepository_NilHub_DoesNotPanic(t *testing.T) {
	base := storage.NewFileRunRepository(t.TempDir())
	repo := NewNotifyingRunRepository(base, nil)

	run := &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusRunning}
	if err := repo.SaveRun(context.Background(), run); err != nil {
		t.Fatalf("SaveRun with nil hub: %v", err)
	}
}
