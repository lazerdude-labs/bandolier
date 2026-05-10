package vault

import "fmt"

// Paths returns Vault KV paths for a given cluster ID. v1 uses a single
// shared KV mount called "bandolier".
type Paths struct{}

func (Paths) Proxmox(clusterID string) string { return fmt.Sprintf("clusters/%s/proxmox", clusterID) }
func (Paths) Network(clusterID string) string { return fmt.Sprintf("clusters/%s/network", clusterID) }
func (Paths) SSH(clusterID string) string     { return fmt.Sprintf("clusters/%s/ssh", clusterID) }
func (Paths) DNS(clusterID string) string     { return fmt.Sprintf("clusters/%s/dns", clusterID) }
func (Paths) TLS(clusterID string) string     { return fmt.Sprintf("clusters/%s/tls", clusterID) }
func (Paths) K3sJoin(clusterID string) string { return fmt.Sprintf("clusters/%s/k3s", clusterID) }
func (Paths) Kubeconfig(clusterID string) string {
	return fmt.Sprintf("clusters/%s/kubeconfig", clusterID)
}
func (Paths) JoinToken(clusterID string) string {
	return fmt.Sprintf("clusters/%s/join_token", clusterID)
}

const KVMount = "bandolier"
