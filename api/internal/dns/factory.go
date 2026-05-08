// api/internal/dns/factory.go
package dns

import "fmt"

// NewProvider returns the Provider impl matching cfg.Kind. Unknown kinds
// return an error rather than nil so the wizard surfaces "unknown DNS
// provider" cleanly to the operator.
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Kind {
	case KindNone:
		return NewNone(), nil
	case KindBind:
		return NewBind(cfg), nil
	case KindPfsense:
		return NewPfsense(cfg), nil
	case KindPihole:
		return NewPihole(cfg), nil
	case KindAdguard:
		return NewAdguard(cfg), nil
	case "":
		return nil, fmt.Errorf("dns: kind not configured")
	default:
		return nil, fmt.Errorf("dns: unknown kind %q", cfg.Kind)
	}
}
