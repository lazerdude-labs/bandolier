package clusters

import (
	"context"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// PurgeVaultSecrets removes the per-cluster KV paths Bandolier writes to.
// Returns a list of human-readable error strings — one per path that failed
// — so the caller can surface them in audit details or logs. A nil vault
// client is a no-op (matches Handler.purgeVaultSecrets's previous behavior;
// used in tests).
//
// Lifted out of *Handler to a package-level fn so both the operator-
// initiated DELETE path AND the executor's cascade-after-destroy path can
// invoke it without the handler dependency.
func PurgeVaultSecrets(ctx context.Context, v *vault.Client, clusterID string) []string {
	if v == nil {
		return nil
	}
	p := vault.Paths{}
	paths := []string{
		p.Proxmox(clusterID),
		p.Network(clusterID),
		p.SSH(clusterID),
		p.K3sJoin(clusterID),
		p.Kubeconfig(clusterID),
		p.JoinToken(clusterID),
		"clusters/" + clusterID + "/wildcard_cert",
	}
	var errs []string
	for _, path := range paths {
		if err := v.Delete(ctx, path); err != nil {
			errs = append(errs, path+": "+err.Error())
		}
	}
	return errs
}

// ForgetCluster orchestrates the full forget flow: Vault path purge,
// SQLite row delete (CASCADE handles deployments / apps_repos /
// apps_installs), audit log entry. Used by both:
//
//  1. The operator-initiated DELETE /api/clusters/{id} handler for clusters
//     already in a non-live state.
//  2. The deployments executor's runDestroy success path, when the cluster
//     was marked pending_forget by a cascade=destroy delete request.
//
// vaultClient may be nil for test contexts. clusterName is best-effort
// metadata for the audit entry; pass "" if unknown.
func ForgetCluster(ctx context.Context, s *store.Store, v *vault.Client,
	clusterID, clusterName, statusAtDelete string, actorID int64) error {
	vaultErrs := PurgeVaultSecrets(ctx, v, clusterID)

	if err := s.DeleteCluster(ctx, clusterID); err != nil {
		_, _ = audit.Write(ctx, s, audit.Entry{
			ActorID: actorID,
			Action:  string(audit.ActionClusterDelete),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "db_error", "error": err.Error()},
		})
		return err
	}

	details := map[string]any{"status_at_delete": statusAtDelete}
	if clusterName != "" {
		details["name"] = clusterName
	}
	if len(vaultErrs) > 0 {
		details["vault_cleanup_errors"] = vaultErrs
	}
	_, _ = audit.Write(ctx, s, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterDelete),
		Target:  clusterID,
		Outcome: audit.OutcomeSuccess,
		Details: details,
	})
	return nil
}
