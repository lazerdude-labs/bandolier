package telemetry

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type Aggregator struct {
	Vault *vault.Client
	Kube  KubeProbe
	Prox  ProxmoxProbe
	cache *cache
	sf    singleflight.Group
}

func NewAggregator(v *vault.Client) *Aggregator {
	return &Aggregator{
		Vault: v,
		cache: newCache(30 * time.Second),
	}
}

// NodeTelemetry returns one row per Kubernetes node, joined with Proxmox VM
// metadata when available. On any partial failure, the corresponding fields
// stay nil (em-dash in UI). Cached for 30s per cluster.
func (a *Aggregator) NodeTelemetry(ctx context.Context, clusterID string) ([]NodeTelemetry, error) {
	if cached, ok := a.cache.get(clusterID); ok {
		return cached, nil
	}

	result, err, _ := a.sf.Do(clusterID, func() (any, error) {
		paths := vault.Paths{}

		// 1. Read kubeconfig from Vault (skip kube probe if missing)
		var rows []NodeTelemetry
		if kc, err := a.Vault.Get(ctx, paths.Kubeconfig(clusterID)); err == nil && kc != nil {
			if yaml, ok := kc["yaml"].(string); ok && yaml != "" {
				if r, err := a.Kube.GetNodes(ctx, yaml); err == nil {
					rows = r
				}
			}
		}

		// 2. Read Proxmox creds and probe VMs (best-effort enrichment)
		if px, err := a.Vault.Get(ctx, paths.Proxmox(clusterID)); err == nil && px != nil {
			creds := ProxmoxCreds{
				Endpoint:    asString(px["endpoint"]),
				TokenID:     asString(px["token_id"]),
				TokenSecret: asString(px["token_secret"]),
				CABundle:    asString(px["ca_bundle"]),
			}
			if creds.Endpoint != "" && creds.TokenID != "" {
				if vms, err := a.Prox.GetVMs(ctx, creds); err == nil {
					rows = enrichWithProxmox(rows, vms)
				}
			}
		}

		// Branched TTL: empty results expire fast (5s) so transient probe
		// failures recover quickly; healthy results cache for 30s.
		if len(rows) == 0 {
			a.cache.putWithTTL(clusterID, rows, 5*time.Second)
		} else {
			a.cache.putWithTTL(clusterID, rows, 30*time.Second)
		}
		return rows, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]NodeTelemetry), nil
}

func enrichWithProxmox(rows []NodeTelemetry, vms []ProxmoxVM) []NodeTelemetry {
	// Match by name (master1 / worker1 / worker2). The terraform module sets
	// the Proxmox VM `name` to the same string we use as the k8s node name.
	byName := make(map[string]ProxmoxVM, len(vms))
	for _, v := range vms {
		byName[v.Name] = v
	}
	for i := range rows {
		if vm, ok := byName[rows[i].Name]; ok {
			node := vm.Node
			vmid := vm.VMID
			rows[i].ProxmoxNode = &node
			rows[i].ProxmoxVMID = &vmid
		}
	}
	return rows
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
