package profiles_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
)

func TestStubProfileMetadata(t *testing.T) {
	p := profiles.NewStub(profiles.Metadata{
		Name: "red-team", Label: "Red Team", Description: "x", Accent: "rose",
	})
	if p.Name() != "red-team" {
		t.Fatalf("name = %q", p.Name())
	}
	if p.Enabled() {
		t.Fatal("stub should not be enabled")
	}
	if p.Metadata().Enabled {
		t.Fatal("stub Metadata.Enabled must be false")
	}
}

func TestStubProfileMethodsReturnErrNotImplemented(t *testing.T) {
	p := profiles.NewStub(profiles.Metadata{Name: "red-team"})
	ctx := context.Background()
	if _, err := p.BuildTfvars(ctx, "x", nil); !errors.Is(err, profiles.ErrNotImplemented) {
		t.Fatalf("BuildTfvars err = %v", err)
	}
	if _, err := p.BuildInventory(ctx, "x", nil, "", nil); !errors.Is(err, profiles.ErrNotImplemented) {
		t.Fatalf("BuildInventory err = %v", err)
	}
}
