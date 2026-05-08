package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	u, err := s.CreateUser(ctx, "hashed-pw")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 || u.PasswordHash != "hashed-pw" {
		t.Fatalf("unexpected user: %+v", u)
	}

	got, err := s.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PasswordHash != "hashed-pw" {
		t.Fatalf("hash mismatch: %s", got.PasswordHash)
	}
}

func TestUserCountIsZeroOnEmptyDB(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	n, err := s.CountUsers(ctx)
	if err != nil || n != 0 {
		t.Fatalf("CountUsers: n=%d err=%v", n, err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	u, _ := s.CreateUser(ctx, "h")

	if err := s.CreateSession(ctx, "sess-id", u.ID, 3600); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := s.GetSession(ctx, "sess-id")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != u.ID {
		t.Fatalf("UserID mismatch")
	}
	if err := s.DeleteSession(ctx, "sess-id"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.GetSession(ctx, "sess-id"); err == nil {
		t.Fatalf("expected error after delete")
	}
}

func TestGetUserByIDNotFoundReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	_, err := s.GetUserByID(ctx, 999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateUserPassword(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	u, err := s.CreateUser(ctx, "$2a$12$old")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateUserPassword(ctx, u.ID, "$2a$12$new"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := s.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordHash != "$2a$12$new" {
		t.Fatalf("hash = %q want $2a$12$new", got.PasswordHash)
	}
}

func TestUpdateUserPasswordReturnsErrNotFoundForMissingID(t *testing.T) {
	s := newStore(t)
	err := s.UpdateUserPassword(context.Background(), 999, "$2a$12$x")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
