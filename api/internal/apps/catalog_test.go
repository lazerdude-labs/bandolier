package apps

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// stubHelm is a no-op Helm implementation used by catalog tests. It satisfies
// just enough of the catalog.Helm contract (RepoAdd / SearchRepo) to drive
// merge + cache behavior without shelling out.
type stubHelm struct {
	repoAddCalls map[string]string
	searchByRepo map[string][]CatalogEntry
	searchErr    error
}

func (s *stubHelm) RepoAdd(ctx context.Context, name, url string) error {
	if s.repoAddCalls == nil {
		s.repoAddCalls = map[string]string{}
	}
	s.repoAddCalls[name] = url
	return nil
}

func (s *stubHelm) RepoRemove(ctx context.Context, name string) error { return nil }
func (s *stubHelm) RepoUpdate(ctx context.Context) error              { return nil }
func (s *stubHelm) SearchRepo(ctx context.Context, name string) ([]CatalogEntry, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchByRepo[name], nil
}
func (s *stubHelm) List(ctx context.Context) ([]Release, error) { return nil, nil }
func (s *stubHelm) Install(ctx context.Context, req InstallRequest, valuesPath string, stdout, stderr io.Writer) error {
	return nil
}
func (s *stubHelm) Upgrade(ctx context.Context, req InstallRequest, valuesPath string, stdout, stderr io.Writer) error {
	return nil
}
func (s *stubHelm) Uninstall(ctx context.Context, releaseName, namespace string, stdout, stderr io.Writer) error {
	return nil
}
func (s *stubHelm) KubeconfigFile() string { return "" }

func TestCatalogMergeCurated(t *testing.T) {
	repoEntries := []CatalogEntry{
		{Source: "bitnami", Name: "grafana", Chart: "bitnami/grafana", LatestVersion: "8.7.0"},
		{Source: "traefik", Name: "traefik", Chart: "traefik/traefik", LatestVersion: "34.2.1"},
	}
	curatedSeed := []CatalogEntry{
		{Source: "curated", Name: "traefik", Chart: "traefik/traefik", LatestVersion: "v34.2.1", System: true, Icon: "shield", Tag: "SYSTEM"},
	}
	merged := mergeCurated(repoEntries, curatedSeed)
	// Curated traefik should win over repo-sourced traefik (same Chart key).
	var sawCurated bool
	var sawRepoTraefik bool
	var sawGrafana bool
	for _, e := range merged {
		if e.Chart == "traefik/traefik" && e.Source == "curated" {
			sawCurated = true
			if !e.System || e.Icon != "shield" {
				t.Fatalf("curated metadata lost: %+v", e)
			}
		}
		if e.Chart == "traefik/traefik" && e.Source == "traefik" {
			sawRepoTraefik = true
		}
		if e.Chart == "bitnami/grafana" {
			sawGrafana = true
		}
	}
	if !sawCurated {
		t.Fatalf("expected curated traefik in merged output: %+v", merged)
	}
	if sawRepoTraefik {
		t.Fatalf("repo-sourced traefik should be replaced by curated entry: %+v", merged)
	}
	if !sawGrafana {
		t.Fatalf("expected bitnami/grafana in merged output: %+v", merged)
	}
}

func TestCatalogCacheTTL(t *testing.T) {
	c := newCatalogCache(50 * time.Millisecond)
	entries := []CatalogEntry{{Source: "bitnami", Name: "x", Chart: "bitnami/x"}}
	c.put("c1", entries)

	got, ok := c.get("c1")
	if !ok || len(got) != 1 {
		t.Fatalf("expected cached entry: %+v ok=%v", got, ok)
	}

	// Different cluster ID — not cached.
	if _, ok := c.get("c2"); ok {
		t.Fatalf("expected cache miss on different key")
	}

	// After TTL, cache should expire.
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.get("c1"); ok {
		t.Fatalf("expected cache expiry")
	}

	// Invalidate clears explicitly.
	c.put("c1", entries)
	c.invalidate("c1")
	if _, ok := c.get("c1"); ok {
		t.Fatalf("expected invalidation to remove entry")
	}
}

func TestCatalogAggregateUsesCache(t *testing.T) {
	// Aggregate without a backing store still returns curated entries and
	// caches them per cluster id. Second call with stub helm in error state
	// should still succeed because of cache hit.
	helm := &stubHelm{
		searchByRepo: map[string][]CatalogEntry{
			"bitnami": {{Source: "bitnami", Name: "x", Chart: "bitnami/x", LatestVersion: "1.0.0"}},
		},
	}
	cat := NewCatalog(nil)
	cat.cache = newCatalogCache(time.Hour)
	out1, err := cat.Aggregate(context.Background(), "c1", helm)
	if err != nil {
		t.Fatal(err)
	}
	if len(out1) == 0 {
		t.Fatalf("expected non-empty aggregation (curated): %+v", out1)
	}

	helm.searchErr = errors.New("should not be called")
	out2, err := cat.Aggregate(context.Background(), "c1", helm)
	if err != nil {
		t.Fatalf("cache hit should mask helm error: %v", err)
	}
	if len(out2) != len(out1) {
		t.Fatalf("cached aggregation drift: %d vs %d", len(out2), len(out1))
	}
}
