package clusters

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/apps"
	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// factoryRepos seeds every newly-created cluster with the curated set of
// upstream Helm repos so the catalog tab is non-empty out of the box. Operators
// can remove any of them via the UI; only the local DB rows + helm repo entries
// are touched (no charts are installed).
var factoryRepos = []struct{ Name, URL string }{
	{Name: "bitnami", URL: "https://charts.bitnami.com/bitnami"},
	{Name: "grafana", URL: "https://grafana.github.io/helm-charts"},
	{Name: "prometheus-community", URL: "https://prometheus-community.github.io/helm-charts"},
	{Name: "traefik", URL: "https://traefik.github.io/charts"},
}

// Sentinel errors used by destroy and downstream handlers.
var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrLocked            = errors.New("cluster is locked by another deployment")
	// ErrClusterNotReady signals the kubeconfig retrieve precondition failed —
	// the cluster exists but its status != "ready". Distinguishes a 400-class
	// user-input issue from infra failures (vault, ssh) that should be 500.
	ErrClusterNotReady = errors.New("cluster not ready for kubeconfig retrieve")
)

type Handler struct {
	store    *store.Store
	registry *profiles.Registry
	vault    *vault.Client
}

func NewHandler(s *store.Store, reg *profiles.Registry, v *vault.Client) *Handler {
	return &Handler{store: s, registry: reg, vault: v}
}

// DestroyExecutor is the minimal subset of *deployments.Executor the destroy handler depends on.
type DestroyExecutor interface {
	Destroy(ctx context.Context, clusterID string) (string, error)
}

type destroyHandler struct {
	store    *store.Store
	executor DestroyExecutor
}

// NewDestroyHandler returns an http.Handler for POST /api/clusters/{id}/destroy.
func NewDestroyHandler(s *store.Store, e DestroyExecutor) http.Handler {
	return &destroyHandler{store: s, executor: e}
}

func (h *destroyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	depID, err := h.executor.Destroy(r.Context(), id)
	switch {
	case errors.Is(err, ErrInvalidTransition):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster cannot be destroyed in current state"})
	case errors.Is(err, ErrLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "deployment in progress"})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "destroy failed"})
	default:
		writeJSON(w, http.StatusAccepted, map[string]string{"deployment_id": depID})
	}
}

type createReq struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// isValidClusterID checks that the URL-supplied cluster ID matches the shape
// newID() produces (32 lowercase hex chars). Defense in depth — the store
// lookup that follows would already reject anything we didn't generate, but
// rejecting at the handler boundary makes the invariant explicit and
// prevents a future refactor that drops the store-first ordering from
// silently letting `../auth/master`-style IDs reach Vault path templates.
func isValidClusterID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		// De Morgan form of !(isDigit || isHexLower) so staticcheck's
		// QF1001 stays quiet.
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())

	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterCreate),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_json"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Name == "" || req.Profile == "" {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterCreate),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "missing_fields"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and profile required"})
		return
	}
	prof, err := h.registry.Get(req.Profile)
	if err != nil {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterCreate),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "unknown_profile"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown profile"})
		return
	}
	if !prof.Enabled() {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterCreate),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "profile_not_implemented"},
		})
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "profile not yet implemented in this release"})
		return
	}
	c := &store.Cluster{
		ID:      newID(),
		Name:    req.Name,
		Profile: req.Profile,
		Status:  string(StatusPending),
	}
	if err := h.store.CreateCluster(r.Context(), c); err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			_, _ = audit.Write(r.Context(), h.store, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionClusterCreate),
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "duplicate_name"},
			})
			writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster name already exists"})
			return
		}
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterCreate),
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "db_error"},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Seed factory-default Helm repos. Best-effort: a failure here doesn't
	// fail cluster create (the operator can re-add the missing repo from the
	// Repos tab), but each failure emits an audit row so we have a paper
	// trail. Loop var named `seed` to avoid shadowing the *http.Request `r`.
	appsStore := apps.NewStore(h.store)
	for _, seed := range factoryRepos {
		if _, err := appsStore.CreateRepo(r.Context(), c.ID, seed.Name, seed.URL, ptrInt64(uid)); err != nil {
			_, _ = audit.Write(r.Context(), h.store, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionAppRepoAdd),
				Target:  c.ID,
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{
					"reason": "seed_failed",
					"name":   seed.Name,
					"url":    seed.URL,
					"error":  err.Error(),
				},
			})
			continue
		}
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoAdd),
			Target:  c.ID,
			Outcome: audit.OutcomeSuccess,
			Details: map[string]any{"name": seed.Name, "url": seed.URL, "factory": true},
		})
	}

	_, _ = audit.Write(r.Context(), h.store, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionClusterCreate),
		Target:  c.ID,
		Outcome: audit.OutcomeSuccess,
		Details: map[string]any{"name": c.Name, "profile": c.Profile},
	})
	writeJSON(w, http.StatusCreated, c)
}

// ptrInt64 returns nil for the zero actor id (anonymous / system caller) and
// a pointer to v otherwise. Mirrors the helper in apps/repos.go but kept
// local to avoid an import cycle around test fixtures.
func ptrInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// enrich produces an EnrichedCluster for the given base cluster, attaching
// the latest deployment row, the static node-count for the profile, and
// best-effort network info read from Vault. Any Vault read failure is
// silently ignored so a cluster with no network secret yet (eg pending,
// pre-initialize) still returns 200 with a nil Network field.
func (h *Handler) enrich(ctx context.Context, c store.Cluster) EnrichedCluster {
	out := EnrichedCluster{Cluster: c}
	n := defaultNodeCount(c.Profile)
	out.NodeCount = &n
	deps, err := h.store.ListDeploymentsForCluster(ctx, c.ID, 1)
	if err == nil && len(deps) > 0 {
		out.LastDeployment = makeLastDeployment(deps[0])
	}
	out.Network = h.readNetwork(ctx, c.ID)
	return out
}

// readNetwork pulls the Vault `clusters/<id>/network` secret if present.
// Returns nil on any error (path not found, vault unreachable, malformed
// data) — callers treat nil as "data unavailable, render em-dash".
func (h *Handler) readNetwork(ctx context.Context, clusterID string) *NetworkInfo {
	if h.vault == nil {
		return nil
	}
	paths := vault.Paths{}
	data, err := h.vault.Get(ctx, paths.Network(clusterID))
	if err != nil || data == nil {
		return nil
	}
	out := &NetworkInfo{
		CIDR:     stringField(data, "cidr"),
		Gateway:  stringField(data, "gateway"),
		FQDN:     stringField(data, "fqdn"),
		MasterIP: stringField(data, "master_ip"),
	}
	if dns, ok := data["dns"].([]any); ok {
		for _, v := range dns {
			if s, ok := v.(string); ok {
				out.DNS = append(out.DNS, s)
			}
		}
	}
	// Initialize wizard stores agent1_ip / agent2_ip as separate keys, not
	// an array. Reconstruct the array for the API surface.
	for _, k := range []string{"agent1_ip", "agent2_ip"} {
		if ip := stringField(data, k); ip != "" {
			out.AgentIPs = append(out.AgentIPs, ip)
		}
	}
	// Phase 4: also pull wildcard cert expiry for the Connection card.
	// Best-effort — pre-Phase-4 clusters won't have this path.
	if certData, err := h.vault.Get(ctx, "clusters/"+clusterID+"/wildcard_cert"); err == nil && certData != nil {
		if expires, ok := certData["expires_at"].(string); ok {
			out.WildcardCertExpiresAt = expires
		}
	}
	return out
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.store.GetCluster(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h.enrich(r.Context(), *c))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	cs, err := h.store.ListClusters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]EnrichedCluster, 0, len(cs))
	for _, c := range cs {
		out = append(out, h.enrich(r.Context(), c))
	}
	writeJSON(w, http.StatusOK, out)
}

// deletableStatuses gates Delete to terminal/idle states only — never a live
// cluster. `destroying`/`upgrading`/`deploying`/`initializing` would orphan
// in-flight infra; `ready`/`degraded` should be destroyed first so terraform
// state stays consistent with reality.
var deletableStatuses = map[string]struct{}{
	string(StatusPending):     {},
	string(StatusInitialized): {},
	string(StatusDestroyed):   {},
	string(StatusError):       {},
}

// Delete removes a cluster's configuration row (and its CASCADE-linked
// deployments / app rows / repo rows) plus best-effort cleanup of its Vault
// secrets. Pure local-state operation — does not touch Proxmox or the VMs.
// Caller must Destroy first if the cluster is ready/degraded.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	c, err := h.store.GetCluster(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterDelete),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "not_found"},
		})
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, ok := deletableStatuses[c.Status]; !ok {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterDelete),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_status", "status": c.Status},
		})
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "destroy the cluster before deleting its configuration",
		})
		return
	}

	// Best-effort Vault cleanup before the DB delete. If Vault is sealed or
	// unreachable we still want the local row gone — operators can reconcile
	// orphaned secrets later via vault CLI. Errors are recorded in audit
	// details but don't block the delete.
	vaultErrs := h.purgeVaultSecrets(r.Context(), id)

	if err := h.store.DeleteCluster(r.Context(), id); err != nil {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterDelete),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "db_error", "error": err.Error()},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	details := map[string]any{"name": c.Name, "profile": c.Profile, "status_at_delete": c.Status}
	if len(vaultErrs) > 0 {
		details["vault_cleanup_errors"] = vaultErrs
	}
	_, _ = audit.Write(r.Context(), h.store, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionClusterDelete),
		Target:  id,
		Outcome: audit.OutcomeSuccess,
		Details: details,
	})
	w.WriteHeader(http.StatusNoContent)
}

// purgeVaultSecrets removes the per-cluster KV paths the rest of the codebase
// writes to (paths.go). Returns a list of human-readable error strings — one
// per path that failed — so the caller can surface them in audit details.
func (h *Handler) purgeVaultSecrets(ctx context.Context, clusterID string) []string {
	if h.vault == nil {
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
		// Phase 4 wildcard cert metadata. Kept inline (not on Paths) to match
		// readNetwork's hand-built path above.
		"clusters/" + clusterID + "/wildcard_cert",
	}
	var errs []string
	for _, path := range paths {
		if err := h.vault.Delete(ctx, path); err != nil {
			errs = append(errs, path+": "+err.Error())
		}
	}
	return errs
}
