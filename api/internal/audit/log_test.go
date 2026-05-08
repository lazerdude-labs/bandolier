package audit_test

import (
	"context"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func TestWriteSuccessRow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, err := audit.Write(ctx, s, audit.Entry{
		ActorID: 1,
		Action:  "change_password",
		Outcome: audit.OutcomeSuccess,
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected id > 0, got %d", id)
	}
	count, err := s.CountAuditEntries(ctx, "change_password", "success")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestWriteFailureWithDetails(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, err := audit.Write(ctx, s, audit.Entry{
		ActorID: 1,
		Action:  "change_password",
		Outcome: audit.OutcomeFailure,
		Details: map[string]any{"reason": "wrong_current_password"},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	count, err := s.CountAuditEntries(ctx, "change_password", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	s, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Pre-seed user id=1 so FK on audit_log.actor_id is satisfied.
	if _, err := s.CreateUser(context.Background(), "hash"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
