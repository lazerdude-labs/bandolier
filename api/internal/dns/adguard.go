// api/internal/dns/adguard.go
package dns

import "context"

type Adguard struct{ cfg Config }

func NewAdguard(cfg Config) *Adguard                           { return &Adguard{cfg: cfg} }
func (a *Adguard) Upsert(_ context.Context, _ Record) error    { return ErrNotImplemented }
func (a *Adguard) Delete(_ context.Context, _, _ string) error { return ErrNotImplemented }
func (a *Adguard) Healthy(_ context.Context) error             { return ErrNotImplemented }
