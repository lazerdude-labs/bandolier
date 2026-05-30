package apps

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// TraefikDefaultChartVersion is the single source of truth for the Traefik
// helm chart version Bandolier ships with. Used both by the curated catalog
// entry below (so the UI shows the correct version) and by the deploy
// executor's traefikChartVersion() helper (the actual install target).
// Cross-package referencing prevents the kind of drift the v0.1.0-v0.1.8
// codebase had — where the executor's hardcoded "34.2.1" was independent
// of any other version source.
const TraefikDefaultChartVersion = "34.5.0"

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
		// Version sourced from TraefikDefaultChartVersion (top of file)
		// so a future bump touches one location. v0.1.x bumped from a
		// never-published "34.2.1" to 34.5.0 — see the const's docstring.
		LatestVersion:     TraefikDefaultChartVersion,
		AvailableVersions: []string{TraefikDefaultChartVersion},
		System:            true,
		IngressValuePath:  "ingress.hostname",
		Icon:              "shield",
		Tag:               "SYSTEM",
	},
	{
		Source: "curated",
		Name:   "homelab-essentials",
		Type:   "bundle",
		// One-line summary; the install modal shows this. Operators see
		// the full per-chart breakdown (release, namespace, hostname,
		// required/optional, helm-chart URL) once they open the modal.
		Description:       "Storage (Longhorn) + observability (kube-prometheus-stack, Loki) + a wiki (Wiki.js). Optional charts can be deselected at install time.",
		// Bundle version is the bundle's own semver, not any single
		// chart's version. v1.0 = the four-chart shape shipped in
		// Bandolier v0.1.12. A future change to the chart list or
		// install ordering bumps this — chart-version bumps within
		// the same shape don't.
		LatestVersion:     "1.0.0",
		AvailableVersions: []string{"1.0.0"},
		Tag:               "BUNDLE",
		Icon:              "box",
		Charts: []BundleChart{
			// 1. Storage. Required because the downstream charts (Prom
			// PVCs, Loki PVCs, Wiki.js Postgres PVC) all want a real
			// StorageClass. k3s's default `local-path` provisioner
			// doesn't survive node loss, which is fine for a demo but
			// wrong for "homelab essentials". Pinning Longhorn as the
			// first chart in install order lets it set itself as the
			// default StorageClass before the others provision PVCs.
			{
				Chart:     "longhorn/longhorn",
				Version:   "1.11.2",
				Release:   "longhorn",
				Namespace: "longhorn-system",
				Hostname:  "{release}.{fqdn}",
				Required:  true,
			},
			// 2. Observability core. Bundles Prometheus + Grafana +
			// Alertmanager + node-exporter + kube-state-metrics in one
			// chart with ServiceMonitors pre-wired. Replaces the old
			// "install Prom + Grafana separately and hope the exporters
			// line up" pattern. Required because most downstream
			// homelab charts assume a working scrape target.
			{
				Chart:     "prometheus-community/kube-prometheus-stack",
				Version:   "85.0.2",
				Release:   "kps",
				Namespace: "monitoring",
				Hostname:  "{release}.{fqdn}",
				Required:  true,
				Storage:   true,
			},
			// 3. Log aggregation. Optional — operators on resource-
			// constrained clusters can skip Loki and rely on
			// `kubectl logs` for the homelab scale they're at.
			// Default chart deploys SimpleScalable mode (3 read/write/
			// backend replicas each). v0.1.12 ships defaults; a future
			// release can carry single-binary values when BundleChart
			// gets a Values field.
			{
				Chart:     "grafana/loki",
				Version:   "7.0.0",
				Release:   "loki",
				Namespace: "monitoring",
				Hostname:  "",
				Required:  false,
				Storage:   true,
			},
			// 4. Wiki / notes. Wiki.js v2 (chart appVersion 2) via the
			// community chart hosted at charts.js.wiki — third-party
			// maintained but at a project-branded URL. Bundles a
			// postgres subchart by default; operators who skip Wiki.js
			// avoid the database footprint entirely. Optional.
			{
				Chart:     "wikijs/wiki",
				Version:   "3.0.0",
				Release:   "wiki",
				Namespace: "wiki",
				Hostname:  "{release}.{fqdn}",
				Required:  false,
				Storage:   true,
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

// catalogRepoLister is the narrow contract Catalog needs from its backing
// store. *Store satisfies this; tests substitute a fake without spinning
// up an actual SQLite-backed store.Store.
type catalogRepoLister interface {
	ListRepos(ctx context.Context, clusterID string) ([]Repo, error)
}

// Catalog assembles curated entries with per-cluster repo-aggregated entries.
// Construct one per process; cache state is internal.
type Catalog struct {
	store catalogRepoLister
	cache *catalogCache
}

// NewCatalog accepts the production *Store. Internal tests construct
// *Catalog directly with a fake repoLister.
//
// Explicit nil-check: passing a typed nil *Store would otherwise produce
// a non-nil interface value (Go's classic interface-nil gotcha), and the
// `c.store != nil` guard inside Aggregate would silently malfunction.
// Tests + main both pass either a real *Store or untyped nil, so keep the
// invariant explicit at construction.
func NewCatalog(s *Store) *Catalog {
	var rs catalogRepoLister
	if s != nil {
		rs = s
	}
	return &Catalog{store: rs, cache: newCatalogCache(catalogCacheTTL)}
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

// aggregateParallelism caps the number of in-flight helm SearchRepo calls
// during a single Aggregate(). 4 is enough to fan out the default seeded
// repos (bitnami, grafana, prometheus-community, traefik) without spawning
// an unbounded goroutine per repo when an operator adds many. Helm CLI
// invocations are not particularly heavy on the api side (they read a
// local index cache, not the remote registry), but each one shells out
// and parses JSON, so bounding the concurrency keeps memory + CPU usage
// predictable.
const aggregateParallelism = 4

// Aggregate returns the merged catalog for a cluster: curated entries plus
// repo-sourced entries from each operator-registered repo. Cached per
// catalogCacheTTL. Best-effort: a single repo failing search does not abort
// the aggregation — those entries are dropped and the rest returned.
//
// Per-repo work runs concurrently behind an errgroup with a concurrency
// cap of aggregateParallelism. RepoAdd + SearchRepo for one repo is one
// task; tasks never return errgroup errors (we always return nil) because
// best-effort semantics mean one bad repo shouldn't fail the whole call.
// Errors from individual repos are logged via slog and the repo is just
// skipped, matching the v0.1.0-v0.1.9 sequential behavior.
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

		// One mutex protects appends to repoEntries from concurrent
		// per-repo goroutines. Pre-sizing the slice based on len(repos)
		// is impossible (we don't know how many charts each repo has),
		// so append-under-lock is simplest. Lock contention is low —
		// each goroutine spends almost all its time in helm CLI I/O
		// and only locks briefly to append the parsed result.
		var mu sync.Mutex
		eg, egCtx := errgroup.WithContext(ctx)
		eg.SetLimit(aggregateParallelism)

		// Run RepoAdd serially BEFORE fanning SearchRepo out in parallel.
		// helm CLI's `repo add` mutates a shared file (~/.config/helm/
		// repositories.yaml). Two goroutines racing to write that file can
		// corrupt it. At 4-goroutine concurrency the practical risk is low,
		// but the 60s aggregate-cache window means the price of one
		// corruption is up to a minute of failed catalog responses — not
		// worth it. SearchRepo is read-only and parallelizes cleanly.
		for _, r := range repos {
			if err := helm.RepoAdd(ctx, r.Name, r.URL); err != nil {
				// Don't log err.Error() verbatim: helm.RepoAdd's error
				// can include the stderr stream from the helm CLI, which
				// on auth failures contains the full repo URL — and a
				// repo URL of the form https://user:password@host/ would
				// leak credentials into structured logs. Log just the
				// fact + repo name; the operator who added the repo
				// can reproduce the failure manually.
				slog.Warn("catalog aggregate: repo add failed (best-effort)",
					"cluster", clusterID, "repo", r.Name)
			}
		}

		for _, r := range repos {
			r := r // capture for goroutine
			eg.Go(func() error {
				entries, err := helm.SearchRepo(egCtx, r.Name)
				if err != nil {
					slog.Warn("catalog aggregate: repo search failed (best-effort, repo skipped)",
						"cluster", clusterID, "repo", r.Name)
					return nil
				}
				mu.Lock()
				repoEntries = append(repoEntries, entries...)
				mu.Unlock()
				return nil
			})
		}
		// errgroup.Wait returns the first non-nil error from any task; we
		// always return nil from tasks so this is purely a fence-and-
		// release on the goroutines. ctx-cancellation surfaces here too.
		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	merged := mergeCurated(repoEntries, curated)
	c.cache.put(clusterID, merged)
	return merged, nil
}

// FilterCatalog applies operator-supplied filter + pagination to an
// already-aggregated catalog slice. Pure function (no IO) — runs against
// the in-memory result of Aggregate, so the 60s aggregate cache absorbs
// the helm CLI cost and per-request filter is fast.
//
// - search: case-insensitive substring match against Name and Description.
//   Empty string means "no search filter".
// - source: exact match against the Source field. Empty string or "all"
//   means "no source filter" (curated AND all repo sources).
// - limit, offset: standard pagination. limit <= 0 means "no pagination"
//   (return everything matching the filters). offset < 0 is clamped to 0.
//
// Returns (entries, totalBeforePagination). Total is the matching count
// before limit/offset are applied, so the UI can render "Showing N of M".
func FilterCatalog(entries []CatalogEntry, search, source string, limit, offset int) ([]CatalogEntry, int) {
	if offset < 0 {
		offset = 0
	}
	q := strings.ToLower(strings.TrimSpace(search))
	matchSource := func(e CatalogEntry) bool {
		return source == "" || source == "all" || e.Source == source
	}
	matchSearch := func(e CatalogEntry) bool {
		if q == "" {
			return true
		}
		return strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Description), q)
	}
	matched := make([]CatalogEntry, 0, len(entries))
	for _, e := range entries {
		if matchSource(e) && matchSearch(e) {
			matched = append(matched, e)
		}
	}
	total := len(matched)
	if offset >= total {
		return []CatalogEntry{}, total
	}
	matched = matched[offset:]
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total
}

// mergeCurated combines repo-derived entries with curated entries. When a
// curated entry matches a repo entry by Chart ("repo/name"), the curated entry
// wins — preserving the curated metadata (Icon, System, IngressValuePath, Tag).
// Output is sorted by (Source, Name) for deterministic rendering.
//
// Bundles (Type == "bundle") set Chart to "" because they don't correspond
// to a single upstream chart — they're a list of charts under one curated
// entry. Without the empty-string skip below, every bundle would register
// the "" key in curatedKeys and shadow any future malformed repo entry
// whose Chart field came back blank from helm search (rare in practice
// but a real edge case for meta-packages). The skip keeps dedup scoped
// to actual chart shadowing.
func mergeCurated(repoEntries, curatedSeed []CatalogEntry) []CatalogEntry {
	curatedKeys := map[string]struct{}{}
	for _, e := range curatedSeed {
		if e.Chart == "" {
			continue
		}
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
