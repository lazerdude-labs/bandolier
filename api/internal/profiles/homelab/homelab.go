package homelab

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type Profile struct {
	terraformDir string
	ansibleDir   string
}

func New(terraformDir, ansibleDir string) *Profile {
	return &Profile{terraformDir: terraformDir, ansibleDir: ansibleDir}
}

func (p *Profile) Name() string                { return "homelab" }
func (p *Profile) TerraformModuleDir() string  { return p.terraformDir }
func (p *Profile) AnsiblePlaybookDir() string  { return p.ansibleDir }
func (p *Profile) AnsiblePlaybookFile() string { return "playbooks/setup.yml" }

func (p *Profile) BuildTfvars(ctx context.Context, clusterID string, vr profiles.VaultReader) (map[string]any, error) {
	paths := vault.Paths{}
	prox, err := vr.Get(ctx, paths.Proxmox(clusterID))
	if err != nil {
		return nil, fmt.Errorf("vault proxmox: %w", err)
	}
	net, err := vr.Get(ctx, paths.Network(clusterID))
	if err != nil {
		return nil, fmt.Errorf("vault network: %w", err)
	}
	ssh, err := vr.Get(ctx, paths.SSH(clusterID))
	if err != nil {
		return nil, fmt.Errorf("vault ssh: %w", err)
	}

	// KV v2 stores dns as []interface{}; terraform expects a comma-separated string.
	dnsList, _ := net["dns"].([]interface{})
	parts := make([]string, len(dnsList))
	for i, d := range dnsList {
		parts[i] = fmt.Sprintf("%v", d)
	}

	distroID, _ := prox["distro"].(string)
	customURL, _ := prox["custom_url"].(string)
	customSHA, _ := prox["custom_sha256"].(string)
	img, err := ResolveImage(distroID, customURL, customSHA)
	if err != nil {
		return nil, fmt.Errorf("resolve image: %w", err)
	}

	// Pick a reachable mirror from the candidate list. If all probes fail
	// (network blip, all mirrors 403, etc.), fall through to the first URL
	// rather than blocking the deploy: the api container's egress isn't
	// guaranteed to match Proxmox's, so a probe failure here doesn't prove
	// Proxmox can't fetch. Operator gets the original behavior + a warning
	// log explaining what we tried.
	imageURL, attempts, perr := PickReachableURL(ctx, img.URLs, nil)
	if perr != nil {
		imageURL = img.URLs[0]
		slog.Warn("image mirror probe failed for all candidates; falling back to first URL",
			"cluster_id", clusterID, "fallback_url", imageURL, "error", perr.Error())
	} else if len(attempts) > 0 {
		// Picked a non-primary; surface the skipped mirrors for operator visibility.
		skipped := make([]string, len(attempts))
		for i, a := range attempts {
			skipped[i] = a.Error()
		}
		slog.Info("image mirror probe selected fallback",
			"cluster_id", clusterID, "selected_url", imageURL, "skipped", skipped)
	}

	imageStorage, _ := prox["image_storage"].(string)
	if imageStorage == "" {
		imageStorage = "local"
	}

	snippetsStorage, _ := prox["snippets_storage"].(string)
	if snippetsStorage == "" {
		snippetsStorage = "local"
	}

	return map[string]any{
		"proxmox_endpoint":         prox["endpoint"],
		"proxmox_token_id":         prox["token_id"],
		"proxmox_token_secret":     prox["token_secret"],
		"proxmox_node":             prox["node"],
		"proxmox_storage":          prox["storage"],
		"proxmox_username":         prox["username"],
		"proxmox_password":         prox["password"],
		"proxmox_image_url":        imageURL,
		"proxmox_image_sha256":     img.SHA256,
		"proxmox_image_filename":   img.FileName,
		"proxmox_image_storage":    imageStorage,
		"proxmox_snippets_storage": snippetsStorage,
		"network_cidr":           net["cidr"],
		"network_bridge_name":    net["bridge_name"],
		"network_gateway":        net["gateway"],
		"network_dns":            strings.Join(parts, ","),
		"network_fqdn":           net["fqdn"],
		"network_master_ip":      net["master_ip"],
		"network_agent1_ip":      net["agent1_ip"],
		"network_agent2_ip":      net["agent2_ip"],
		"network_vlan":           net["vlan"],
		"ssh_public_key":         ssh["public_key"],
		"cluster_id":             clusterID,
	}, nil
}

func (p *Profile) BuildInventory(ctx context.Context, clusterID string, tfOutputs map[string]any, runDir string, vr profiles.VaultReader) (string, error) {
	paths := vault.Paths{}
	net, err := vr.Get(ctx, paths.Network(clusterID))
	if err != nil {
		return "", fmt.Errorf("vault network: %w", err)
	}
	sshSecret, err := vr.Get(ctx, paths.SSH(clusterID))
	if err != nil {
		return "", fmt.Errorf("vault ssh: %w", err)
	}

	// Write private key to runDir so ansible can use it. runDir is cleaned up
	// by the executor's defer os.RemoveAll.
	sshKeyPath := filepath.Join(runDir, "ssh_key")
	privateKey, _ := sshSecret["private_key"].(string)
	if err := os.WriteFile(sshKeyPath, []byte(privateKey), 0o600); err != nil {
		return "", fmt.Errorf("write ssh key: %w", err)
	}

	master := net["master_ip"]
	a1 := net["agent1_ip"]
	a2 := net["agent2_ip"]

	// Group names mirror ansible/playbooks/setup.yml: hosts: server, agent, k3s
	return fmt.Sprintf(`[server]
master ansible_host=%v ansible_user=ansible ansible_ssh_private_key_file=%s

[agent]
agent1 ansible_host=%v ansible_user=ansible ansible_ssh_private_key_file=%s
agent2 ansible_host=%v ansible_user=ansible ansible_ssh_private_key_file=%s

[k3s:children]
server
agent

[all:vars]
ansible_python_interpreter=/usr/bin/python3
`, master, sshKeyPath, a1, sshKeyPath, a2, sshKeyPath), nil
}

func (p *Profile) BuildExtraVars(ctx context.Context, clusterID string, vr profiles.VaultReader) (map[string]any, error) {
	paths := vault.Paths{}
	net, err := vr.Get(ctx, paths.Network(clusterID))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"cluster_id":   clusterID,
		"k3s_version":  "v1.31.12+k3s1",
		"fqdn":         net["fqdn"],
		"api_endpoint": net["master_ip"], // k3s_agent role needs this to know the server
		"api_port":     6443,
	}, nil
}

// BuildUpgradeVars reuses BuildExtraVars and overrides k3s_version with the
// operator-supplied target. The upgrade playbook reads the same vars as setup.
func (p *Profile) BuildUpgradeVars(ctx context.Context, clusterID, k3sVersion string, vr profiles.VaultReader) (map[string]any, error) {
	base, err := p.BuildExtraVars(ctx, clusterID, vr)
	if err != nil {
		return nil, err
	}
	base["k3s_version"] = k3sVersion
	return base, nil
}

func (p *Profile) Enabled() bool { return true }

func (p *Profile) Metadata() profiles.Metadata {
	return profiles.Metadata{
		Name:        "homelab",
		Label:       "Homelab",
		Description: "Single-server, 2-agent k3s cluster on Proxmox.",
		Accent:      "emerald",
		Tag:         "PRODUCTION",
		Icon:        "shield",
		Enabled:     true,
	}
}

// PreDestroy is a no-op for homelab — terraform destroy needs the same Vault values
// that BuildTfvars already supplies.
func (p *Profile) PreDestroy(_ context.Context, _ string, _ profiles.VaultReader) error {
	return nil
}

// PostDestroy zeroes the cluster's k3s join token and kubeconfig in Vault.
// Best-effort: both deletes are attempted and errors joined, so a transient
// failure on one path does not strand the other. Proxmox/network/SSH paths
// are retained for redeploy without re-entering credentials.
func (p *Profile) PostDestroy(ctx context.Context, clusterID string, vw profiles.VaultWriter) error {
	paths := vault.Paths{}
	var errs []error
	if err := vw.Delete(ctx, paths.K3sJoin(clusterID)); err != nil {
		errs = append(errs, fmt.Errorf("delete k3s join: %w", err))
	}
	if err := vw.Delete(ctx, paths.Kubeconfig(clusterID)); err != nil {
		errs = append(errs, fmt.Errorf("delete kubeconfig: %w", err))
	}
	return errors.Join(errs...)
}
