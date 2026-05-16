package clusters_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// newReconcileStore stands up a real SQLite store with the full migration
// set and an apps.Store on top. Uses an on-disk file (not :memory:) because
// the migration runner expects a real path; cleanup via t.TempDir handles
// it. Matches the pattern used by other store-touching tests in this
// codebase.
func newReconcileStore(t *testing.T) (*store.Store, *apps.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, apps.NewStore(s)
}

func mustCluster(t *testing.T, s *store.Store, id, name string) {
	t.Helper()
	if err := s.CreateCluster(context.Background(), &store.Cluster{
		ID: id, Name: name, Profile: "homelab", Status: "initialized",
	}); err != nil {
		t.Fatalf("CreateCluster %s: %v", id, err)
	}
}

func mustRepo(t *testing.T, as *apps.Store, clusterID, name, url string) {
	t.Helper()
	if _, err := as.CreateRepo(context.Background(), clusterID, name, url, nil); err != nil {
		t.Fatalf("CreateRepo %s/%s: %v", clusterID, name, err)
	}
}

func repoNames(t *testing.T, as *apps.Store, clusterID string) map[string]bool {
	t.Helper()
	rs, err := as.ListRepos(context.Background(), clusterID)
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	out := map[string]bool{}
	for _, r := range rs {
		out[r.Name] = true
	}
	return out
}

// TestReconcileFactoryReposAddsMissingOnly asserts the post-v0.1.12 fix:
// a pre-v0.1.12 cluster (only the original four factory repos) gets the
// new longhorn + wikijs entries on the next api boot. Non-factory repos
// the operator added themselves are left alone.
func TestReconcileFactoryReposAddsMissingOnly(t *testing.T) {
	s, as := newReconcileStore(t)
	mustCluster(t, s, "01234567890123456789012345678901", "pre-v0112")

	// Pre-v0.1.12 baseline: the original four factory repos.
	mustRepo(t, as, "01234567890123456789012345678901", "bitnami", "https://charts.bitnami.com/bitnami")
	mustRepo(t, as, "01234567890123456789012345678901", "grafana", "https://grafana.github.io/helm-charts")
	mustRepo(t, as, "01234567890123456789012345678901", "prometheus-community", "https://prometheus-community.github.io/helm-charts")
	mustRepo(t, as, "01234567890123456789012345678901", "traefik", "https://traefik.github.io/charts")
	// Plus a custom operator-added repo that's NOT in FactoryRepos. Reconciler
	// must not touch this entry.
	mustRepo(t, as, "01234567890123456789012345678901", "operator-custom", "https://example.test/charts")

	if err := clusters.ReconcileFactoryRepos(context.Background(), s, as); err != nil {
		t.Fatalf("ReconcileFactoryRepos: %v", err)
	}

	got := repoNames(t, as, "01234567890123456789012345678901")
	for _, fr := range clusters.FactoryRepos {
		if !got[fr.Name] {
			t.Errorf("expected factory repo %q after reconcile; got %v", fr.Name, got)
		}
	}
	if !got["operator-custom"] {
		t.Error("operator-custom repo was removed by reconciler — must be additive only")
	}
}

// TestReconcileFactoryReposIsIdempotent ensures a second reconcile run
// adds nothing — the first call fully reconciled the cluster.
func TestReconcileFactoryReposIsIdempotent(t *testing.T) {
	s, as := newReconcileStore(t)
	mustCluster(t, s, "00000000000000000000000000000001", "idem-cluster")

	// First pass: cluster has no repos at all (worst case for a brand-new
	// cluster that somehow skipped Create's seeding).
	if err := clusters.ReconcileFactoryRepos(context.Background(), s, as); err != nil {
		t.Fatalf("first ReconcileFactoryRepos: %v", err)
	}
	firstPass := repoNames(t, as, "00000000000000000000000000000001")
	if len(firstPass) != len(clusters.FactoryRepos) {
		t.Fatalf("expected %d repos after first reconcile, got %d (%v)",
			len(clusters.FactoryRepos), len(firstPass), firstPass)
	}

	// Second pass: should be a no-op. CreateRepo enforces UNIQUE(cluster_id,
	// name) so a regression that re-inserts would surface as a DB error
	// the reconciler logs and skips — but the result here would still be
	// a stable set, so we assert on stability.
	if err := clusters.ReconcileFactoryRepos(context.Background(), s, as); err != nil {
		t.Fatalf("second ReconcileFactoryRepos: %v", err)
	}
	secondPass := repoNames(t, as, "00000000000000000000000000000001")
	if len(secondPass) != len(firstPass) {
		t.Errorf("idempotency violated: pass 1 had %d repos, pass 2 had %d", len(firstPass), len(secondPass))
	}
}

// TestReconcileFactoryReposSkipsOnEmptyClusters confirms the reconciler
// does no harm against a store with zero clusters — important because
// boot ordering means this runs before any user-driven cluster create.
func TestReconcileFactoryReposSkipsOnEmptyClusters(t *testing.T) {
	s, as := newReconcileStore(t)
	if err := clusters.ReconcileFactoryRepos(context.Background(), s, as); err != nil {
		t.Fatalf("ReconcileFactoryRepos on empty store: %v", err)
	}
}
