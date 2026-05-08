// api/internal/dns/pihole.go
package dns

import "context"

type Pihole struct{ cfg Config }

func NewPihole(cfg Config) *Pihole                            { return &Pihole{cfg: cfg} }
func (p *Pihole) Upsert(_ context.Context, _ Record) error    { return ErrNotImplemented }
func (p *Pihole) Delete(_ context.Context, _, _ string) error { return ErrNotImplemented }
func (p *Pihole) Healthy(_ context.Context) error             { return ErrNotImplemented }
