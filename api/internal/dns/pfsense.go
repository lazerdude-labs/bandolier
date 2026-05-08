// api/internal/dns/pfsense.go
package dns

import "context"

// Pfsense is a stub adapter awaiting implementation. Operators picking
// "pfsense" at cluster initialize time will see a clear ErrNotImplemented
// error from the wizard's Test connection button rather than a confusing
// runtime failure.
type Pfsense struct{ cfg Config }

func NewPfsense(cfg Config) *Pfsense                           { return &Pfsense{cfg: cfg} }
func (p *Pfsense) Upsert(_ context.Context, _ Record) error    { return ErrNotImplemented }
func (p *Pfsense) Delete(_ context.Context, _, _ string) error { return ErrNotImplemented }
func (p *Pfsense) Healthy(_ context.Context) error             { return ErrNotImplemented }
