package auth_test

import (
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/auth"
)

func TestValidatePasswordRejectsShort(t *testing.T) {
	if err := auth.ValidatePassword("short"); err == nil {
		t.Fatal("expected error for <12 char password")
	}
}

func TestValidatePasswordAcceptsLong(t *testing.T) {
	if err := auth.ValidatePassword("twelvechars12"); err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
}
