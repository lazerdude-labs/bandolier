package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func TestListAuditEntriesFilters(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	s, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	// Need a user for FK
	if _, err := s.CreateUser(ctx, "$2a$12$x"); err != nil {
		t.Fatal(err)
	}

	// Seed 4 entries: 2 different actions, 2 different outcomes
	for _, e := range []store.AuditEntry{
		{ActorID: 1, Action: "auth_login", Outcome: "success"},
		{ActorID: 1, Action: "auth_login", Outcome: "failure"},
		{ActorID: 1, Action: "cluster_deploy", Outcome: "started"},
		{ActorID: 1, Action: "cluster_deploy", Outcome: "succeeded"},
	} {
		if _, err := s.InsertAuditEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	// No filter -> 4 rows
	rows, err := s.ListAuditEntries(ctx, store.AuditFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("want 4, got %d", len(rows))
	}

	// Filter by action
	rows, err = s.ListAuditEntries(ctx, store.AuditFilter{Action: "auth_login", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("auth_login want 2, got %d", len(rows))
	}

	// Filter by outcome
	rows, err = s.ListAuditEntries(ctx, store.AuditFilter{Outcome: "failure", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("failure want 1, got %d", len(rows))
	}

	// Filter by since (future)
	future := time.Now().Add(time.Hour)
	rows, err = s.ListAuditEntries(ctx, store.AuditFilter{Since: &future, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("future since want 0, got %d", len(rows))
	}
}
