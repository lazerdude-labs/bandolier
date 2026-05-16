package clusters

import (
	"context"
	"log/slog"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// ReconcileFactoryRepos walks every cluster in the store and inserts any
// FactoryRepos entry that's missing from its apps_repos rows. Run at api
// boot time after store + appsStore are initialized.
//
// Why this exists: FactoryRepos seeding only runs at cluster-create time
// (see Handler.Create). Pre-v0.1.12 clusters were created when the seed
// list had four entries (bitnami / grafana / prometheus-community /
// traefik). v0.1.12 added longhorn + wikijs to support the
// homelab-essentials bundle; without this reconciler, those pre-v0.1.12
// clusters never get the new repos and any bundle-install attempt fails
// at `helm install longhorn/longhorn` with "repo longhorn not found".
//
// Reconciliation is purely additive — repos the operator explicitly
// removed via the Repos tab will be re-added. That's a known minor
// regression vs the v0.1.12 design: if an operator decided to drop
// `bitnami` from their cluster, this reconciler puts it back at the
// next api boot. Acceptable for v0.1.13 because (a) factory repos are
// the curated baseline, (b) the operator can re-remove via the Repos
// tab at any time, (c) the failure mode of NOT having this is a hard
// bundle-install failure with no clear operator-facing diagnostic.
// A future release could introduce an "operator deleted this factory
// repo, don't restore" tombstone if the regression matters in practice.
//
// Idempotent: a second call adds nothing because every factory repo
// is already present after the first call.
//
// Per-cluster errors are logged and the loop continues — one bad
// cluster (e.g. orphaned DB row pointing at a deleted Vault path)
// shouldn't block reconciliation for everything else.
func ReconcileFactoryRepos(ctx context.Context, s *store.Store, appsStore *apps.Store) error {
	clusters, err := s.ListClusters(ctx)
	if err != nil {
		return err
	}
	added := 0
	for _, c := range clusters {
		existing, err := appsStore.ListRepos(ctx, c.ID)
		if err != nil {
			slog.Warn("factory repo reconcile: list repos failed",
				"cluster", c.ID, "err", err.Error())
			continue
		}
		existingNames := make(map[string]bool, len(existing))
		for _, r := range existing {
			existingNames[r.Name] = true
		}
		for _, fr := range FactoryRepos {
			if existingNames[fr.Name] {
				continue
			}
			if _, err := appsStore.CreateRepo(ctx, c.ID, fr.Name, fr.URL, nil); err != nil {
				// Log without err.Error() verbatim — CreateRepo can wrap
				// SQLite errors but we're not aware of a credential-leak
				// path here. Keep the err message but bound it.
				slog.Warn("factory repo reconcile: add failed",
					"cluster", c.ID, "repo", fr.Name, "err", err.Error())
				continue
			}
			added++
			slog.Info("factory repo reconcile: added",
				"cluster", c.ID, "repo", fr.Name)
		}
	}
	if added > 0 {
		slog.Info("factory repo reconcile: complete", "added", added, "clusters", len(clusters))
	}
	return nil
}
