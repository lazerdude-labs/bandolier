package apps

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// releasesCacheTTL is the per-cluster Helm-list cache window. Helm list is a
// remote call (via shell-out + kubeconfig) so cache to keep page loads
// snappy without staling out beyond what operators can tolerate.
const releasesCacheTTL = 30 * time.Second

// releaseCache mirrors catalogCache but caches []Release per cluster id. Kept
// private to the package; install/upgrade/uninstall handlers call invalidate
// after operations through the Handler.ReleasesCache accessor.
type releaseCache struct {
	ttl  time.Duration
	mu   sync.Mutex
	data map[string]releaseCacheEntry
}

type releaseCacheEntry struct {
	at       time.Time
	releases []Release
}

func newReleaseCache(ttl time.Duration) *releaseCache {
	return &releaseCache{ttl: ttl, data: map[string]releaseCacheEntry{}}
}

func (c *releaseCache) get(key string) ([]Release, bool) {
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
	out := make([]Release, len(e.releases))
	copy(out, e.releases)
	return out, true
}

func (c *releaseCache) put(key string, releases []Release) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]Release, len(releases))
	copy(cp, releases)
	c.data[key] = releaseCacheEntry{at: time.Now(), releases: cp}
}

func (c *releaseCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Handler serves the read endpoints over apps_installs and the live helm
// state. Mutating ops (install/upgrade/uninstall) live in a separate executor
// (Phase 3C) which uses ReleasesCache().invalidate to drop stale cache rows
// after a successful operation.
type Handler struct {
	store    *Store
	catalog  *Catalog
	hf       HelmFactory
	releases *releaseCache
}

func NewHandler(s *Store, catalog *Catalog, hf HelmFactory) *Handler {
	return &Handler{
		store:    s,
		catalog:  catalog,
		hf:       hf,
		releases: newReleaseCache(releasesCacheTTL),
	}
}

// ReleasesCache exposes the internal release cache so the Phase 3C install
// executor can invalidate after a write. Returning the unexported type is
// intentional — callers in this package only.
func (h *Handler) ReleasesCache() *releaseCache { return h.releases }

// Catalog returns the merged curated + repo-aggregated catalog for a
// cluster, optionally filtered + paginated server-side.
//
// GET /api/clusters/{id}/apps/catalog
//
// Query params (all optional):
//
//   - search:  case-insensitive substring match against name + description
//   - source:  exact match against entry source ("curated" or repo name).
//              Empty or "all" disables the filter.
//   - limit:   max entries returned, 0-500. 0 / unset means no limit.
//   - offset:  pagination offset, >= 0.
//
// Response: {"entries": [...], "total": N} where N is the count of all
// matching entries before pagination. Lets the UI show "Showing X of N".
func (h *Handler) Catalog(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")

	q := r.URL.Query()
	search := q.Get("search")
	source := q.Get("source")
	limit, ok := parseBoundedInt(q.Get("limit"), 0, 500)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be 0-500"})
		return
	}
	offset, ok := parseBoundedInt(q.Get("offset"), 0, 1<<20)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "offset must be 0 or positive"})
		return
	}

	helm, cleanup, err := h.hf.For(r.Context(), clusterID)
	if err != nil {
		// No kubeconfig (cluster not ready) — return curated only so the
		// UI has something to render rather than a 500. Repos require a
		// working cluster anyway. Apply the same filter+paginate path so
		// the UI's "Showing N of M" stays correct against the curated
		// subset.
		curated := h.catalog.Curated()
		entries, total := FilterCatalog(curated, search, source, limit, offset)
		writeJSON(w, http.StatusOK, CatalogResponse{Entries: entries, Total: total})
		return
	}
	defer cleanup()

	all, err := h.catalog.Aggregate(r.Context(), clusterID, helm)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}
	if all == nil {
		all = []CatalogEntry{}
	}
	entries, total := FilterCatalog(all, search, source, limit, offset)
	if entries == nil {
		entries = []CatalogEntry{}
	}
	writeJSON(w, http.StatusOK, CatalogResponse{Entries: entries, Total: total})
}

// parseBoundedInt parses a string into an int and validates it falls
// within [min, max] inclusive. Empty string is treated as 0 (the
// "unset" sentinel for limit/offset). Returns (value, ok); ok=false
// means the caller should 400.
func parseBoundedInt(s string, min, max int) (int, bool) {
	if s == "" {
		return 0, true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	if n < min || n > max {
		return 0, false
	}
	return n, true
}

// Releases returns the live Helm release list for a cluster, cached 30s.
//
// GET /api/clusters/{id}/apps/releases
func (h *Handler) Releases(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")

	if cached, ok := h.releases.get(clusterID); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	helm, cleanup, err := h.hf.For(r.Context(), clusterID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cluster helm unavailable"})
		return
	}
	defer cleanup()

	releases, err := helm.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}
	if releases == nil {
		releases = []Release{}
	}
	h.releases.put(clusterID, releases)
	writeJSON(w, http.StatusOK, releases)
}

// Installs returns the install history rows for a cluster (most recent first).
// Optional ?limit query (default 50, range 1-200). Validated at the HTTP
// boundary so a malicious or buggy caller can't force a large DB scan via
// `?limit=99999999` — pattern matches Catalog's parseBoundedInt gate.
//
// GET /api/clusters/{id}/apps/installs
func (h *Handler) Installs(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, ok := parseBoundedInt(v, 1, 200)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be 1-200"})
			return
		}
		limit = n
	}
	rows, err := h.store.ListInstallsForCluster(r.Context(), clusterID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}
	if rows == nil {
		rows = []Install{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// GetInstall returns a single install record by id.
//
// GET /api/apps/installs/{id}
func (h *Handler) GetInstall(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	in, err := h.store.GetInstall(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "install not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}
	writeJSON(w, http.StatusOK, in)
}

// ExecHandler serves the mutating endpoints (install/upgrade/uninstall). It
// invalidates the per-cluster Helm release cache after each accepted request
// so the next /releases call re-fetches from helm rather than serving stale
// data. The caller subscribes to /ws/apps/installs/{id}/logs to follow the
// async operation.
type ExecHandler struct {
	store    *Store
	exec     *Executor
	releases *releaseCache
}

func NewExecHandler(s *Store, e *Executor, releases *releaseCache) *ExecHandler {
	return &ExecHandler{store: s, exec: e, releases: releases}
}

// Install kicks off a Helm install. Body: InstallRequest. Returns 202 with the
// install id; logs stream over the WS route. 409 when another op holds the
// per-cluster mutex.
//
// POST /api/clusters/{id}/apps/install
func (h *ExecHandler) Install(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.Chart = strings.TrimSpace(req.Chart)
	req.Version = strings.TrimSpace(req.Version)
	req.ReleaseName = strings.TrimSpace(req.ReleaseName)
	req.Namespace = strings.TrimSpace(req.Namespace)
	if req.Chart == "" || req.Version == "" || req.ReleaseName == "" || req.Namespace == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chart, version, release_name, namespace required"})
		return
	}

	id, err := h.exec.Install(r.Context(), clusterID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrLocked):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster busy"})
		default:
			writeJSON(w, http.StatusInternalServerError, errMap(err))
		}
		return
	}
	if h.releases != nil {
		h.releases.invalidate(clusterID)
	}
	writeJSON(w, http.StatusAccepted, InstallResponse{InstallID: id})
}

// Upgrade kicks off a Helm upgrade for the named release. Body: InstallRequest
// (release_name in the URL takes precedence). Same accept/error semantics as
// Install.
//
// POST /api/clusters/{id}/apps/{release}/upgrade
func (h *ExecHandler) Upgrade(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	release := chi.URLParam(r, "release")
	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.ReleaseName = strings.TrimSpace(release)
	req.Chart = strings.TrimSpace(req.Chart)
	req.Version = strings.TrimSpace(req.Version)
	req.Namespace = strings.TrimSpace(req.Namespace)
	if req.Chart == "" || req.Version == "" || req.Namespace == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "chart, version, namespace required"})
		return
	}

	id, err := h.exec.Upgrade(r.Context(), clusterID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrLocked):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster busy"})
		default:
			writeJSON(w, http.StatusInternalServerError, errMap(err))
		}
		return
	}
	if h.releases != nil {
		h.releases.invalidate(clusterID)
	}
	writeJSON(w, http.StatusAccepted, InstallResponse{InstallID: id})
}

type uninstallReq struct {
	Namespace string `json:"namespace"`
	Force     bool   `json:"force"`
}

// Uninstall kicks off a Helm uninstall for the named release.
// 400 for system releases (e.g. traefik), 409 when locked.
//
// POST /api/clusters/{id}/apps/{release}/uninstall
func (h *ExecHandler) Uninstall(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	release := chi.URLParam(r, "release")
	var req uninstallReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	if req.Namespace == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "namespace required"})
		return
	}

	id, err := h.exec.Uninstall(r.Context(), clusterID, release, req.Namespace, req.Force)
	if err != nil {
		switch {
		case errors.Is(err, ErrSystemRel):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, ErrLocked):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster busy"})
		default:
			writeJSON(w, http.StatusInternalServerError, errMap(err))
		}
		return
	}
	if h.releases != nil {
		h.releases.invalidate(clusterID)
	}
	writeJSON(w, http.StatusAccepted, InstallResponse{InstallID: id})
}

// InstallBundleHandler kicks off a multi-chart bundle install. Body:
// BundleInstallRequest. Returns 202 with the install id; logs stream over the
// shared WS route. 404 when the cluster is unknown; 409 when another op holds
// the per-cluster mutex.
//
// POST /api/clusters/{id}/apps/bundle
func (h *ExecHandler) InstallBundleHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req BundleInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Bundle == "" || req.Version == "" || len(req.Choices) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bundle, version, choices required"})
		return
	}
	if _, err := h.exec.Core.GetCluster(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
		return
	}
	netData, _ := h.exec.Vault.Get(r.Context(), "clusters/"+id+"/network")
	fqdn, _ := netData["fqdn"].(string)
	installID, err := h.exec.InstallBundle(r.Context(), id, req, fqdn)
	switch {
	case errors.Is(err, ErrLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "deployment in progress"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, errMap(err))
	default:
		if h.releases != nil {
			h.releases.invalidate(id)
		}
		writeJSON(w, http.StatusAccepted, InstallResponse{InstallID: installID})
	}
}

