package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// vaultHelmFactory is the production HelmFactory: it fetches the cluster's
// kubeconfig from Vault, writes it to an ephemeral 0600 temp file, and hands
// the path to a HelmCLI bound to that file. The cleanup func removes the temp
// directory. Always invoke cleanup — even on error paths inside the caller.
type vaultHelmFactory struct {
	vault  *vault.Client
	binary string
}

// NewVaultHelmFactory returns a HelmFactory that materializes a per-cluster
// kubeconfig from Vault for each invocation. helmBinary may be empty — HelmCLI
// falls back to "helm" on PATH.
func NewVaultHelmFactory(v *vault.Client, helmBinary string) HelmFactory {
	return &vaultHelmFactory{vault: v, binary: helmBinary}
}

func (f *vaultHelmFactory) For(ctx context.Context, clusterID string) (Helm, func(), error) {
	paths := vault.Paths{}
	data, err := f.vault.Get(ctx, paths.Kubeconfig(clusterID))
	if err != nil {
		return nil, func() {}, fmt.Errorf("read kubeconfig from vault: %w", err)
	}
	rawYAML, _ := data["yaml"].(string)
	if rawYAML == "" {
		return nil, func() {}, fmt.Errorf("kubeconfig for cluster %q missing or empty", clusterID)
	}

	dir, err := os.MkdirTemp("", "helm-kc-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create temp kubeconfig dir: %w", err)
	}
	kc := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(kc, []byte(rawYAML), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, func() {}, fmt.Errorf("write temp kubeconfig: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }
	return HelmCLI{Binary: f.binary, KubeconfigPath: kc}, cleanup, nil
}
