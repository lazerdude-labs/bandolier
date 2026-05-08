package profiles

import "context"

// Profile describes one cluster type. v1 has only "homelab"; v3 will add red/blue.
type Profile interface {
	Name() string
	Enabled() bool
	Metadata() Metadata

	TerraformModuleDir() string
	AnsiblePlaybookDir() string
	AnsiblePlaybookFile() string

	// BuildTfvars builds the variables map for terraform from Vault secrets.
	BuildTfvars(ctx context.Context, clusterID string, vault VaultReader) (map[string]any, error)

	// BuildInventory builds an Ansible inventory from terraform outputs and writes
	// the SSH private key into runDir/ssh_key (mode 0600). The returned inventory
	// string references that path.
	BuildInventory(ctx context.Context, clusterID string, tfOutputs map[string]any, runDir string, vault VaultReader) (string, error)

	// BuildExtraVars builds the Ansible --extra-vars JSON.
	BuildExtraVars(ctx context.Context, clusterID string, vault VaultReader) (map[string]any, error)

	// BuildUpgradeVars returns the extra-vars map used when running the
	// upgrade playbook. It mirrors BuildExtraVars but overrides k3s_version
	// with the target requested by the operator.
	BuildUpgradeVars(ctx context.Context, clusterID, k3sVersion string, vault VaultReader) (map[string]any, error)

	PreDestroy(ctx context.Context, clusterID string, vault VaultReader) error
	PostDestroy(ctx context.Context, clusterID string, vault VaultWriter) error
}

type VaultReader interface {
	Get(ctx context.Context, path string) (map[string]any, error)
}
