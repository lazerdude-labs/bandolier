// api/internal/dns/conformance_test.go
package dns

import (
	"context"
	"errors"
	"testing"
)

func TestProvidersConform(t *testing.T) {
	var _ Provider = NewBind(Config{})
	var _ Provider = NewPfsense(Config{})
	var _ Provider = NewPihole(Config{})
	var _ Provider = NewAdguard(Config{})
	var _ Provider = NewNone()
}

func TestNoneAlwaysSucceeds(t *testing.T) {
	n := NewNone()
	if err := n.Upsert(context.Background(), Record{}); err != nil {
		t.Errorf("Upsert: %v", err)
	}
	if err := n.Delete(context.Background(), "", ""); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if err := n.Healthy(context.Background()); err != nil {
		t.Errorf("Healthy: %v", err)
	}
}

func TestStubsReturnNotImplemented(t *testing.T) {
	stubs := []Provider{NewPfsense(Config{}), NewPihole(Config{}), NewAdguard(Config{})}
	for _, p := range stubs {
		if err := p.Upsert(context.Background(), Record{}); !errors.Is(err, ErrNotImplemented) {
			t.Errorf("%T.Upsert: want ErrNotImplemented, got %v", p, err)
		}
		if err := p.Delete(context.Background(), "", ""); !errors.Is(err, ErrNotImplemented) {
			t.Errorf("%T.Delete: want ErrNotImplemented, got %v", p, err)
		}
		if err := p.Healthy(context.Background()); !errors.Is(err, ErrNotImplemented) {
			t.Errorf("%T.Healthy: want ErrNotImplemented, got %v", p, err)
		}
	}
}
