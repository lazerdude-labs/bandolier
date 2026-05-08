package greyspace

import (
	"context"

	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles/homelab"
)

// Profile is the grey-space target profile. v3 metadata-only differentiation:
// shares the homelab provisioning pipeline via delegation; only Name() and
// Metadata() are scenario-specific.
type Profile struct {
	delegate *homelab.Profile
}

func New(terraformDir, ansibleDir string) *Profile {
	return &Profile{delegate: homelab.New(terraformDir, ansibleDir)}
}

func (p *Profile) Name() string                { return "grey-space" }
func (p *Profile) Enabled() bool               { return true }
func (p *Profile) TerraformModuleDir() string  { return p.delegate.TerraformModuleDir() }
func (p *Profile) AnsiblePlaybookDir() string  { return p.delegate.AnsiblePlaybookDir() }
func (p *Profile) AnsiblePlaybookFile() string { return p.delegate.AnsiblePlaybookFile() }

func (p *Profile) BuildTfvars(ctx context.Context, clusterID string, vr profiles.VaultReader) (map[string]any, error) {
	return p.delegate.BuildTfvars(ctx, clusterID, vr)
}

func (p *Profile) BuildInventory(ctx context.Context, clusterID string, tfOut map[string]any, runDir string, vr profiles.VaultReader) (string, error) {
	return p.delegate.BuildInventory(ctx, clusterID, tfOut, runDir, vr)
}

func (p *Profile) BuildExtraVars(ctx context.Context, clusterID string, vr profiles.VaultReader) (map[string]any, error) {
	return p.delegate.BuildExtraVars(ctx, clusterID, vr)
}

func (p *Profile) BuildUpgradeVars(ctx context.Context, clusterID, k3sVersion string, vr profiles.VaultReader) (map[string]any, error) {
	return p.delegate.BuildUpgradeVars(ctx, clusterID, k3sVersion, vr)
}

func (p *Profile) PreDestroy(ctx context.Context, clusterID string, vr profiles.VaultReader) error {
	return p.delegate.PreDestroy(ctx, clusterID, vr)
}

func (p *Profile) PostDestroy(ctx context.Context, clusterID string, vw profiles.VaultWriter) error {
	return p.delegate.PostDestroy(ctx, clusterID, vw)
}

func (p *Profile) Metadata() profiles.Metadata {
	return profiles.Metadata{
		Name:        "grey-space",
		Label:       "Grey Space",
		Description: "Vulnerable target cluster.",
		Accent:      "amber",
		Tag:         "TARGET",
		Icon:        "alert-triangle",
		Enabled:     true,
	}
}
