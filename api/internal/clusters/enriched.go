package clusters

import (
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// LastDeployment is a thin projection of the most recent deployment row
// for a cluster, surfaced via the cluster API so the UI can render
// "last deploy: 4h ago · succeeded" without a second round-trip.
type LastDeployment struct {
	ID        string    `json:"id"`
	Operation string    `json:"operation"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// NetworkInfo carries best-effort values read from Vault by the cluster
// handler. Fields are populated only when the corresponding Vault path
// exists; otherwise the field is the zero value (rendered as em-dash by
// the UI).
type NetworkInfo struct {
	CIDR                  string   `json:"cidr,omitempty"`
	Gateway               string   `json:"gateway,omitempty"`
	DNS                   []string `json:"dns,omitempty"`
	FQDN                  string   `json:"fqdn,omitempty"`
	MasterIP              string   `json:"master_ip,omitempty"`
	AgentIPs              []string `json:"agent_ips,omitempty"`
	WildcardCertExpiresAt string   `json:"wildcard_cert_expires_at,omitempty"`
}

// EnrichedCluster is the JSON shape returned by GET /api/clusters/{id}
// and (in slice form) by GET /api/clusters. It embeds the base cluster
// row and adds derived/optional fields. Pointer fields are nil when the
// data is unavailable; the UI renders nil as em-dash.
type EnrichedCluster struct {
	store.Cluster
	NodeCount      *int            `json:"node_count"`
	LastDeployment *LastDeployment `json:"last_deployment"`
	Network        *NetworkInfo    `json:"network"`
	// K3sVersion is nil until per-cluster k3s telemetry is wired (Plan 2 phase 2).
	K3sVersion *string `json:"k3s_version"`
}

// defaultNodeCount returns the static node count for a profile when the
// real number isn't available via inventory or live telemetry. Used for
// rendering the fleet table's Nodes column. v3 stub profiles return 0
// because they cannot be deployed; rendered as "0" or em-dash.
func defaultNodeCount(profile string) int {
	switch profile {
	case "homelab":
		return 3
	default:
		return 0
	}
}

// makeLastDeployment converts a store.Deployment row into the API-facing
// projection. Returns nil if the deployment has no started_at (shouldn't
// happen for committed rows, but defensive).
func makeLastDeployment(d store.Deployment) *LastDeployment {
	if !d.StartedAt.Valid {
		return nil
	}
	return &LastDeployment{
		ID:        d.ID,
		Operation: d.Operation,
		Status:    d.Status,
		StartedAt: d.StartedAt.Time,
	}
}
