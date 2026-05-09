package deployments

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// FetchKubeconfig SSHes to the master node, reads /etc/rancher/k3s/k3s.yaml,
// rewrites the server address, and writes it to Vault.
//
// sshKeyPath: path to a tmpfs-mounted private key already used by the deploy.
// masterIP: IP to substitute for 127.0.0.1 / 0.0.0.0 in the kubeconfig.
// sshUser: user to log in as (typically "ansible" or "rocky").
func FetchKubeconfig(ctx context.Context, vaultClient *vault.Client, clusterID, sshKeyPath, sshUser, masterIP string) error {
	target := fmt.Sprintf("%s@%s", sshUser, masterIP)
	cmd := exec.CommandContext(ctx,
		"ssh",
		"-i", sshKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		target,
		"sudo cat /etc/rancher/k3s/k3s.yaml",
	)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ssh fetch kubeconfig: %w", err)
	}
	yaml := rewriteServerAddress(string(out), masterIP)

	paths := vault.Paths{}
	if err := vaultClient.Put(ctx, paths.Kubeconfig(clusterID), map[string]any{
		"yaml":         yaml,
		"retrieved_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("vault put kubeconfig: %w", err)
	}
	return nil
}

// rewriteServerAddress replaces 127.0.0.1 and 0.0.0.0 in the YAML's `server:`
// field with the cluster's master IP. Done as a string replace rather than a
// YAML parse to keep dependencies minimal and tolerate formatting variants.
func rewriteServerAddress(yaml, masterIP string) string {
	yaml = strings.ReplaceAll(yaml, "https://127.0.0.1:6443", "https://"+masterIP+":6443")
	yaml = strings.ReplaceAll(yaml, "https://0.0.0.0:6443", "https://"+masterIP+":6443")
	return yaml
}

// RetrieveKubeconfig is the manual-retry path: same as the auto-fetch in
// runDeploy but invoked from a handler. Requires cluster.status == "ready".
// The runDir from the original deploy is gone, so SSH key + master IP are
// re-read from Vault and the key is materialised to a short-lived temp file.
func (e *Executor) RetrieveKubeconfig(ctx context.Context, clusterID string) error {
	c, err := e.Store.GetCluster(ctx, clusterID)
	if err != nil {
		return err
	}
	if c.Status != "ready" {
		return fmt.Errorf("%w (got %s)", clusters.ErrClusterNotReady, c.Status)
	}
	paths := vault.Paths{}
	net, err := e.Vault.Get(ctx, paths.Network(clusterID))
	if err != nil {
		return fmt.Errorf("vault network: %w", err)
	}
	masterIP, _ := net["master_ip"].(string)
	if masterIP == "" {
		return fmt.Errorf("master_ip missing from vault network")
	}
	ssh, err := e.Vault.Get(ctx, paths.SSH(clusterID))
	if err != nil {
		return fmt.Errorf("vault ssh: %w", err)
	}
	privateKey, _ := ssh["private_key"].(string)
	if privateKey == "" {
		return fmt.Errorf("ssh private key missing")
	}
	tmpDir, err := os.MkdirTemp("", "kubeconfig-retrieve-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	keyPath := filepath.Join(tmpDir, "ssh_key")
	if err := os.WriteFile(keyPath, []byte(privateKey), 0o600); err != nil {
		return fmt.Errorf("write ssh key: %w", err)
	}
	return FetchKubeconfig(ctx, e.Vault, clusterID, keyPath, "ansible", masterIP)
}
