package apps

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
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

// TestCatalogMergeCuratedBundleNoChartShadow pins the v0.1.12 fix that
// bundle entries (Type == "bundle", Chart == "") don't register the
// empty string as a dedup key. Without the fix, a malformed repo entry
// returning an empty Chart would be silently swallowed by mergeCurated's
// shadowing logic.
func TestCatalogMergeCuratedBundleNoChartShadow(t *testing.T) {
	repoEntries := []CatalogEntry{
		{Source: "bitnami", Name: "well-formed", Chart: "bitnami/well-formed", LatestVersion: "1.0.0"},
		// Simulate a malformed helm-search row: empty Chart. Should NOT
		// be shadowed by the bundle below.
		{Source: "bitnami", Name: "edge-case", Chart: "", LatestVersion: "1.0.0"},
	}
	curatedSeed := []CatalogEntry{
		{Source: "curated", Name: "homelab-essentials", Type: "bundle", Chart: ""},
	}
	merged := mergeCurated(repoEntries, curatedSeed)

	var sawEdgeCase bool
	for _, e := range merged {
		if e.Name == "edge-case" {
			sawEdgeCase = true
		}
	}
	if !sawEdgeCase {
		t.Fatalf("repo entry with empty Chart was incorrectly shadowed by bundle: %+v", merged)
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

// TestFilterCatalog walks the filter+pagination matrix. Pure-function helper
// so no setup beyond an input slice.
func TestFilterCatalog(t *testing.T) {
	all := []CatalogEntry{
		{Source: "curated", Name: "traefik", Description: "Default ingress controller for Bandolier clusters."},
		{Source: "curated", Name: "homelab-essentials", Description: "Curated bundle of starter apps."},
		{Source: "bitnami", Name: "nginx", Description: "NGINX web server."},
		{Source: "bitnami", Name: "postgres", Description: "PostgreSQL database."},
		{Source: "bitnami", Name: "redis", Description: "In-memory data store."},
		{Source: "grafana", Name: "grafana", Description: "Open source visualization."},
	}
	cases := []struct {
		name        string
		search      string
		source      string
		limit       int
		offset      int
		wantNames   []string
		wantTotal   int
	}{
		{"no filter, no paginate", "", "", 0, 0, []string{"traefik", "homelab-essentials", "nginx", "postgres", "redis", "grafana"}, 6},
		{"source=curated", "", "curated", 0, 0, []string{"traefik", "homelab-essentials"}, 2},
		{"source=all is equivalent to no source", "", "all", 0, 0, []string{"traefik", "homelab-essentials", "nginx", "postgres", "redis", "grafana"}, 6},
		{"search=post matches postgres only", "post", "", 0, 0, []string{"postgres"}, 1},
		{"search is case-insensitive", "PoSt", "", 0, 0, []string{"postgres"}, 1},
		{"search matches description too", "ingress", "", 0, 0, []string{"traefik"}, 1},
		{"search + source combined", "g", "bitnami", 0, 0, []string{"nginx", "postgres"}, 2},
		{"limit truncates result", "", "bitnami", 2, 0, []string{"nginx", "postgres"}, 3},
		{"offset advances", "", "bitnami", 0, 1, []string{"postgres", "redis"}, 3},
		{"limit + offset together", "", "bitnami", 1, 1, []string{"postgres"}, 3},
		{"offset >= total returns empty but real total", "", "bitnami", 0, 99, []string{}, 3},
		{"no match", "zzzzz-no-match", "", 0, 0, []string{}, 0},
		{"negative offset clamps to 0", "", "curated", 0, -5, []string{"traefik", "homelab-essentials"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, total := FilterCatalog(all, tc.search, tc.source, tc.limit, tc.offset)
			if total != tc.wantTotal {
				t.Errorf("total: got %d want %d", total, tc.wantTotal)
			}
			if len(got) != len(tc.wantNames) {
				t.Fatalf("count: got %d want %d (names=%v)", len(got), len(tc.wantNames), namesOf(got))
			}
			for i, want := range tc.wantNames {
				if got[i].Name != want {
					t.Errorf("entry %d: got %q want %q", i, got[i].Name, want)
				}
			}
		})
	}
}

func namesOf(es []CatalogEntry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

// blockingHelm counts concurrent in-flight SearchRepo calls. Used to verify
// that Aggregate's errgroup concurrency cap is respected. The arrival
// channel fires once per goroutine entering SearchRepo, before it blocks
// on the gate — lets the test confirm the expected concurrency level
// before releasing, removing all timing flakiness.
type blockingHelm struct {
	stubHelm
	arrive    chan struct{} // signaled when a goroutine enters SearchRepo
	gate      chan struct{} // closes to release blocked goroutines
	concur    atomic.Int32
	maxConcur atomic.Int32
}

func (b *blockingHelm) SearchRepo(ctx context.Context, name string) ([]CatalogEntry, error) {
	now := b.concur.Add(1)
	defer b.concur.Add(-1)
	// Track high-water mark.
	for {
		peak := b.maxConcur.Load()
		if now <= peak || b.maxConcur.CompareAndSwap(peak, now) {
			break
		}
	}
	b.arrive <- struct{}{}
	<-b.gate
	return []CatalogEntry{
		{Source: name, Name: name + "-chart", Chart: name + "/" + name + "-chart"},
	}, nil
}

// TestAggregateRespectsConcurrencyCap asserts that more than
// aggregateParallelism repos don't all run helm SearchRepo simultaneously.
// Dispatches 8 repos against a blocking stub, waits for exactly
// aggregateParallelism goroutines to confirm in-flight via the arrive
// channel, then releases the gate. The wait is the synchronization point
// — no time.Sleep, no flakiness on slow CI runners.
func TestAggregateRespectsConcurrencyCap(t *testing.T) {
	st := &fakeRepoStore{}
	for i := 0; i < 8; i++ {
		st.repos = append(st.repos, Repo{Name: "repo-" + string(rune('a'+i)), URL: "https://example.test/charts"})
	}
	cat := &Catalog{store: st, cache: newCatalogCache(time.Hour)}
	helm := &blockingHelm{
		arrive: make(chan struct{}, 16),
		gate:   make(chan struct{}),
	}

	// Wait for exactly aggregateParallelism goroutines to be in-flight,
	// then release. Confirms the cap is observed (extra goroutines stay
	// queued behind the errgroup's SetLimit gate, not in SearchRepo).
	released := make(chan struct{})
	go func() {
		for i := 0; i < aggregateParallelism; i++ {
			<-helm.arrive
		}
		// Briefly let any 5th goroutine try to enter — if SetLimit is
		// broken, it would arrive and bump maxConcur past the cap.
		// Drain any spurious arrivals via a short non-blocking peek
		// before releasing.
		select {
		case <-helm.arrive:
			// More than aggregateParallelism in-flight — let it through,
			// the maxConcur assertion below will catch it.
			helm.arrive <- struct{}{} // restore so SearchRepo can re-emit on next call
		case <-time.After(50 * time.Millisecond):
		}
		close(helm.gate)
		close(released)
	}()

	if _, err := cat.Aggregate(context.Background(), "c1", helm); err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	<-released

	peak := helm.maxConcur.Load()
	if peak <= 0 {
		t.Fatalf("expected peak concurrency > 0, got %d", peak)
	}
	if int(peak) > aggregateParallelism {
		t.Errorf("peak concurrency %d exceeded cap %d", peak, aggregateParallelism)
	}
}

// fakeRepoStore satisfies the subset of *store.Store that Catalog needs
// (ListRepos). Defined here rather than reused from store_test.go to keep
// the dependency direction simple — catalog tests should not depend on
// store internals beyond the interface they actually use.
type fakeRepoStore struct {
	mu    sync.Mutex
	repos []Repo
}

func (s *fakeRepoStore) ListRepos(ctx context.Context, clusterID string) ([]Repo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Repo, len(s.repos))
	copy(out, s.repos)
	return out, nil
}
