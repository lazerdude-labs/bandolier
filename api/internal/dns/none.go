// api/internal/dns/none.go
package dns

import "context"

// None is the no-op DNS provider used when an operator opts out of
// Bandolier-managed DNS. All methods return nil — Upsert/Delete are
// silent successes; Healthy passes immediately. The deploy goroutine's
// dns.write_wildcard step runs to completion as a no-op.
type None struct{}

func NewNone() *None { return &None{} }

func (n *None) Upsert(_ context.Context, _ Record) error    { return nil }
func (n *None) Delete(_ context.Context, _, _ string) error { return nil }
func (n *None) Healthy(_ context.Context) error             { return nil }
