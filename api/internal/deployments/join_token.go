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

// FetchJoinToken SSHes to the master node, reads the k3s server node-token,
// and writes it to Vault under clusters/<id>/join_token.
//
// sshKeyPath: path to a tmpfs-mounted private key already used by the deploy.
// sshUser:    user to log in as (typically "ansible").
// masterIP:   the control-plane node IP.
func FetchJoinToken(ctx context.Context, vaultClient *vault.Client, clusterID, sshKeyPath, sshUser, masterIP string) error {
	target := fmt.Sprintf("%s@%s", sshUser, masterIP)
	cmd := exec.CommandContext(ctx,
		"ssh",
		"-i", sshKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		target,
		"sudo cat /var/lib/rancher/k3s/server/node-token",
	)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ssh fetch join token: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return fmt.Errorf("join token empty")
	}

	paths := vault.Paths{}
	if err := vaultClient.Put(ctx, paths.JoinToken(clusterID), map[string]any{
		"token":        token,
		"retrieved_at": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return fmt.Errorf("vault put join token: %w", err)
	}
	return nil
}

// RetrieveJoinToken is the manual-retry path: same as the auto-fetch in
// runDeploy but invoked from a handler. Requires cluster.status == "ready".
// The runDir from the original deploy is gone, so SSH key + master IP are
// re-read from Vault and the key is materialised to a short-lived temp file.
func (e *Executor) RetrieveJoinToken(ctx context.Context, clusterID string) error {
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
	tmpDir, err := os.MkdirTemp("", "join-token-retrieve-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	keyPath := filepath.Join(tmpDir, "ssh_key")
	if err := os.WriteFile(keyPath, []byte(privateKey), 0o600); err != nil {
		return fmt.Errorf("write ssh key: %w", err)
	}
	return FetchJoinToken(ctx, e.Vault, clusterID, keyPath, "ansible", masterIP)
}
