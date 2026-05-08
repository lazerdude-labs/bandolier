package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type NodeTelemetry struct {
	Name         string     `json:"name"`
	Role         string     `json:"role"` // server | agent — set by aggregator from inventory
	IP           string     `json:"ip"`
	K3sVersion   *string    `json:"k3s_version"`
	Ready        *bool      `json:"ready"`
	LastHealthAt *time.Time `json:"last_health_at"`
	ProxmoxNode  *string    `json:"proxmox_node"`
	ProxmoxVMID  *int       `json:"proxmox_vmid"`
}

type KubeProbe struct{}

// GetNodes runs kubectl against an in-memory kubeconfig YAML and returns one
// row per Kubernetes node. The kubeconfig is written to a temp file in a
// freshly-created MkdirTemp directory and removed before this function
// returns.
func (KubeProbe) GetNodes(ctx context.Context, kubeconfigYAML string) ([]NodeTelemetry, error) {
	dir, err := os.MkdirTemp("", "kubeconfig-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	kc := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(kc, []byte(kubeconfigYAML), 0o600); err != nil {
		return nil, fmt.Errorf("write kubeconfig: %w", err)
	}

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kc, "get", "nodes", "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get nodes: %w", err)
	}
	return parseKubeNodes(out)
}

type kubeAddr struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}
type kubeCond struct {
	Type              string `json:"type"`
	Status            string `json:"status"`
	LastHeartbeatTime string `json:"lastHeartbeatTime"`
}
type kubeNode struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Addresses  []kubeAddr `json:"addresses"`
		Conditions []kubeCond `json:"conditions"`
		NodeInfo   struct {
			KubeletVersion string `json:"kubeletVersion"`
		} `json:"nodeInfo"`
	} `json:"status"`
}
type kubeList struct {
	Items []kubeNode `json:"items"`
}

func parseKubeNodes(raw []byte) ([]NodeTelemetry, error) {
	var list kubeList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse kubectl output: %w", err)
	}
	out := make([]NodeTelemetry, 0, len(list.Items))
	for _, n := range list.Items {
		row := NodeTelemetry{Name: n.Metadata.Name}
		for _, a := range n.Status.Addresses {
			if a.Type == "InternalIP" {
				row.IP = a.Address
			}
		}
		if v := n.Status.NodeInfo.KubeletVersion; v != "" {
			row.K3sVersion = &v
		}
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" {
				ready := c.Status == "True"
				row.Ready = &ready
				if t, err := time.Parse(time.RFC3339, c.LastHeartbeatTime); err == nil {
					row.LastHealthAt = &t
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}
