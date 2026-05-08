// api/internal/dns/types.go
package dns

import "errors"

// Kind enumerates the supported DNS provider implementations. BIND9 is the
// only working impl in v1; the rest are stubs awaiting contribution.
type Kind string

const (
	KindNone    Kind = "none"
	KindBind    Kind = "bind"
	KindPfsense Kind = "pfsense"
	KindPihole  Kind = "pihole"
	KindAdguard Kind = "adguard"
)

// Config carries the per-cluster DNS authority configuration. Persisted in
// Vault at clusters/<id>/dns. Fields not relevant to a given Kind are ignored.
type Config struct {
	Kind       Kind   `json:"kind"`
	Server     string `json:"server"`      // "192.0.2.5:53"
	Zone       string `json:"zone"`        // "lab.local"
	TSIGName   string `json:"tsig_name"`   // BIND-only
	TSIGSecret string `json:"tsig_secret"` // BIND-only
	APIToken   string `json:"api_token"`   // pfsense/pihole/adguard
}

// Record is the unit of DNS state Bandolier writes. Phase 4 only writes A
// records (the wildcard at cluster create); the type is broader so future
// provider impls don't need to widen the interface.
type Record struct {
	Name string // "*.homelab.lab.local." (trailing dot per RFC 1035)
	Type string // "A" (only A in v1)
	TTL  int    // seconds; default 300 if zero
	Data string // "192.0.2.21"
}

// ErrNotImplemented is returned by stub provider impls. Callers can errors.Is
// against it to distinguish "operator picked a provider we haven't built yet"
// from real network/auth failures.
var ErrNotImplemented = errors.New("dns: provider not implemented")
