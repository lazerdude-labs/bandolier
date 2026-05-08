package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	var version int
	if err := s.DB().QueryRowContext(context.Background(),
		`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != 6 {
		t.Fatalf("expected schema_version=6, got %d", version)
	}
}
