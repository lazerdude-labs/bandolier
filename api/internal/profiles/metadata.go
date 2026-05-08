package profiles

import "context"

type Metadata struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Accent      string `json:"accent"`
	// Tag is a one-word category label rendered as an uppercase pill in the UI.
	// Conventionally one of: PRODUCTION, SCENARIO, TARGET.
	Tag string `json:"tag"`
	// Icon is a lucide-react icon name (kebab-case, eg "shield", "alert-triangle").
	// The UI maps these to React components.
	Icon    string `json:"icon"`
	Enabled bool   `json:"enabled"`
}

// VaultWriter is implemented by *vault.Client.
type VaultWriter interface {
	VaultReader
	Delete(ctx context.Context, path string) error
}
