// api/internal/dns/provider.go
package dns

import "context"

// Provider is the DNS authority surface Bandolier uses. Implementations
// connect to BIND9, pfSense, Pi-hole, etc. Stubs return ErrNotImplemented
// for every method but satisfy the interface so consumers can depend on
// it abstractly.
type Provider interface {
	// Upsert creates or replaces a record. Implementations must be
	// idempotent — calling Upsert twice with the same Record is a no-op
	// after the first.
	Upsert(ctx context.Context, r Record) error

	// Delete removes the record matching name + type. Best-effort: no
	// error for "record didn't exist" — that's success from the caller's
	// perspective.
	Delete(ctx context.Context, name, recordType string) error

	// Healthy verifies Bandolier can reach the provider AND has the auth
	// material to write. Used by the initialize wizard's Test connection
	// button. Returns nil when ready; error with a human-readable reason
	// when not.
	Healthy(ctx context.Context) error
}
