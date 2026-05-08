package profiles

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("profile not implemented")

type StubProfile struct {
	meta Metadata
}

func NewStub(m Metadata) *StubProfile {
	m.Enabled = false
	return &StubProfile{meta: m}
}

func (s *StubProfile) Name() string                { return s.meta.Name }
func (s *StubProfile) Enabled() bool               { return false }
func (s *StubProfile) Metadata() Metadata          { return s.meta }
func (s *StubProfile) TerraformModuleDir() string  { return "" }
func (s *StubProfile) AnsiblePlaybookDir() string  { return "" }
func (s *StubProfile) AnsiblePlaybookFile() string { return "" }

func (s *StubProfile) BuildTfvars(ctx context.Context, clusterID string, vr VaultReader) (map[string]any, error) {
	return nil, ErrNotImplemented
}

func (s *StubProfile) BuildInventory(ctx context.Context, clusterID string, tfOutputs map[string]any, runDir string, vr VaultReader) (string, error) {
	return "", ErrNotImplemented
}

func (s *StubProfile) BuildExtraVars(ctx context.Context, clusterID string, vr VaultReader) (map[string]any, error) {
	return nil, ErrNotImplemented
}

func (s *StubProfile) BuildUpgradeVars(ctx context.Context, clusterID, k3sVersion string, vr VaultReader) (map[string]any, error) {
	return nil, ErrNotImplemented
}

func (s *StubProfile) PreDestroy(ctx context.Context, clusterID string, vr VaultReader) error {
	return ErrNotImplemented
}

func (s *StubProfile) PostDestroy(ctx context.Context, clusterID string, vw VaultWriter) error {
	return ErrNotImplemented
}
