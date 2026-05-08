package apps

import (
	"context"
	"io"
	"sort"
	"sync"
	"time"
)

// Helm is the abstract surface the catalog/handlers use to talk to helm. The
// concrete HelmCLI shell-out wrapper in helm.go satisfies this; tests
// substitute a stub.
type Helm interface {
	RepoAdd(ctx context.Context, name, url string) error
	RepoRemove(ctx context.Context, name string) error
	RepoUpdate(ctx context.Context) error
	SearchRepo(ctx context.Context, name string) ([]CatalogEntry, error)
	List(ctx context.Context) ([]Release, error)
	Install(ctx context.Context, req InstallRequest, valuesPath string, stdout, stderr io.Writer) error
	Upgrade(ctx context.Context, req InstallRequest, valuesPath string, stdout, stderr io.Writer) error
	Uninstall(ctx context.Context, releaseName, namespace string, stdout, stderr io.Writer) error
	// KubeconfigFile returns the on-disk path of the temp kubeconfig this
	// Helm wrapper is bound to. Exposed so deploy-time helpers (e.g.
	// pushing a TLS Secret via kubectl) can target the same cluster
	// without redoing the vault → temp-file dance.
	KubeconfigFile() string
}

// curated holds the in-memory catalog entries Bandolier ships with. v1: just
// Traefik so the deploy goroutine can use the catalog as the source of truth
// for the system ingress install.
var curated = []CatalogEntry{
	{
		Source:            "curated",
		Name:              "traefik",
		Chart:             "traefik/traefik",
		Type:              "chart",
		Description:       "Default ingress controller for Bandolier clusters.",
		LatestVersion:     "34.2.1",
		AvailableVersions: []string{"34.2.1"},
		System:            true,
		IngressValuePath:  "ingress.hostname",
		Icon:              "shield",
		Tag:               "SYSTEM",
	},
	{
		Source:            "curated",
		Name:              "homelab-starter",
		Type:              "bundle",
		Description:       "Stub bundle. Real curated bundles ship in Phase 5.",
		LatestVersion:     "v0.1",
		AvailableVersions: []string{"v0.1"},
		Tag:               "BUNDLE",
		Charts: []BundleChart{
			{
				Chart:     "bitnami/nginx",
				Version:   "18.1.13",
				Release:   "demo-nginx",
				Namespace: "default",
				Hostname:  "{release}.{fqdn}",
				Required:  true,
			},
		},
	},
}

// catalogCacheTTL is the per-cluster aggregated catalog cache window. Operators
// don't add a chart every page load — 60s is plenty for the catalog tab. Made
// package-level so tests can override (and so production has one place to
// tune it).
const catalogCacheTTL = 60 * time.Second

// catalogCache holds aggregated catalog entries keyed by cluster id with a
// TTL. Concurrency-safe; methods take the internal mutex.
type catalogCache struct {
	ttl  time.Duration
	mu   sync.Mutex
	data map[string]catalogCacheEntry
}

type catalogCacheEntry struct {
	at      time.Time
	entries []CatalogEntry
}

func newCatalogCache(ttl time.Duration) *catalogCache {
	return &catalogCache{ttl: ttl, data: map[string]catalogCacheEntry{}}
}

func (c *catalogCache) get(key string) ([]CatalogEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok {
		return nil, false
	}
	if time.Since(e.at) > c.ttl {
		delete(c.data, key)
		return nil, false
	}
	// Return a defensive copy so callers can't mutate the cache.
	out := make([]CatalogEntry, len(e.entries))
	copy(out, e.entries)
	return out, true
}

func (c *catalogCache) put(key string, entries []CatalogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]CatalogEntry, len(entries))
	copy(cp, entries)
	c.data[key] = catalogCacheEntry{at: time.Now(), entries: cp}
}

func (c *catalogCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Catalog assembles curated entries with per-cluster repo-aggregated entries.
// Construct one per process; cache state is internal.
type Catalog struct {
	store *Store
	cache *catalogCache
}

func NewCatalog(s *Store) *Catalog {
	return &Catalog{store: s, cache: newCatalogCache(catalogCacheTTL)}
}

// Curated returns a defensive copy of the curated catalog slice.
func (c *Catalog) Curated() []CatalogEntry {
	out := make([]CatalogEntry, len(curated))
	copy(out, curated)
	return out
}

// FindCurated returns the curated entry by chart name (the leaf — "traefik"
// not "traefik/traefik"). Returns false if absent.
func (c *Catalog) FindCurated(name string) (CatalogEntry, bool) {
	for _, e := range curated {
		if e.Name == name {
			return e, true
		}
	}
	return CatalogEntry{}, false
}

// Invalidate clears the cache slot for a cluster — call after repo add/remove.
func (c *Catalog) Invalidate(clusterID string) { c.cache.invalidate(clusterID) }

// Aggregate returns the merged catalog for a cluster: curated entries plus
// repo-sourced entries from each operator-registered repo. Cached per
// catalogCacheTTL. Best-effort: a single repo failing search does not abort
// the aggregation — those entries are dropped and the rest returned.
func (c *Catalog) Aggregate(ctx context.Context, clusterID string, helm Helm) ([]CatalogEntry, error) {
	if cached, ok := c.cache.get(clusterID); ok {
		return cached, nil
	}

	var repoEntries []CatalogEntry
	if c.store != nil {
		repos, err := c.store.ListRepos(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		for _, r := range repos {
			// Best-effort: ensure the repo is present in helm's local list.
			// Errors are non-fatal — search will fail and we'll skip below.
			_ = helm.RepoAdd(ctx, r.Name, r.URL)
			entries, err := helm.SearchRepo(ctx, r.Name)
			if err != nil {
				continue
			}
			repoEntries = append(repoEntries, entries...)
		}
	}

	merged := mergeCurated(repoEntries, curated)
	c.cache.put(clusterID, merged)
	return merged, nil
}

// mergeCurated combines repo-derived entries with curated entries. When a
// curated entry matches a repo entry by Chart ("repo/name"), the curated entry
// wins — preserving the curated metadata (Icon, System, IngressValuePath, Tag).
// Output is sorted by (Source, Name) for deterministic rendering.
func mergeCurated(repoEntries, curatedSeed []CatalogEntry) []CatalogEntry {
	curatedKeys := map[string]struct{}{}
	for _, e := range curatedSeed {
		curatedKeys[e.Chart] = struct{}{}
	}
	out := make([]CatalogEntry, 0, len(curatedSeed)+len(repoEntries))
	out = append(out, curatedSeed...)
	for _, e := range repoEntries {
		if _, shadowed := curatedKeys[e.Chart]; shadowed {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			// curated first, then repo names alphabetically.
			if out[i].Source == "curated" {
				return true
			}
			if out[j].Source == "curated" {
				return false
			}
			return out[i].Source < out[j].Source
		}
		return out[i].Name < out[j].Name
	})
	return out
}
