package apps

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"context"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// HelmFactory produces a Helm wrapper bound to a cluster's kubeconfig. The
// cleanup func is responsible for tearing down whatever ephemeral state the
// factory created (typically removing a temp kubeconfig file). It is always
// safe to call, even on error paths.
type HelmFactory interface {
	For(ctx context.Context, clusterID string) (Helm, func(), error)
}

// RepoHandler serves HTTP for the per-cluster apps repos collection. All
// mutating ops emit audit rows (success/failure) using the existing one-shot
// audit pattern.
type RepoHandler struct {
	store   *Store
	core    *store.Store
	catalog *Catalog
	hf      HelmFactory
}

func NewRepoHandler(s *Store, core *store.Store, catalog *Catalog, hf HelmFactory) *RepoHandler {
	return &RepoHandler{store: s, core: core, catalog: catalog, hf: hf}
}

// List returns operator-registered repos for a cluster.
//
// GET /api/clusters/{id}/apps/repos
func (h *RepoHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	repos, err := h.store.ListRepos(r.Context(), clusterID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}
	if repos == nil {
		repos = []Repo{}
	}
	writeJSON(w, http.StatusOK, repos)
}

type addRepoReq struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Add registers a new helm repo for the cluster. Body: {name, url}. Calls
// helm repo add against the cluster's kubeconfig before persisting; if the
// helm call fails, no DB row is created.
//
// POST /api/clusters/{id}/apps/repos
func (h *RepoHandler) Add(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	uid, _ := auth.UserIDFromContext(r.Context())

	var req addRepoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoAdd),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "invalid_json"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	if req.Name == "" || req.URL == "" {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoAdd),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "missing_fields", "name": req.Name, "url": req.URL},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and url required"})
		return
	}

	helm, cleanup, err := h.hf.For(r.Context(), clusterID)
	if err != nil {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoAdd),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "helm_unavailable", "error": err.Error(), "name": req.Name, "url": req.URL},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "helm unavailable"})
		return
	}
	defer cleanup()

	if err := helm.RepoAdd(r.Context(), req.Name, req.URL); err != nil {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoAdd),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "helm_repo_add_failed", "error": err.Error(), "name": req.Name, "url": req.URL},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "helm repo add failed: " + err.Error()})
		return
	}

	id, err := h.store.CreateRepo(r.Context(), clusterID, req.Name, req.URL, ptrInt64(uid))
	if err != nil {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoAdd),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "db_error", "error": err.Error(), "name": req.Name, "url": req.URL},
		})
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}

	if h.catalog != nil {
		h.catalog.Invalidate(clusterID)
	}

	_, _ = audit.Write(r.Context(), h.core, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionAppRepoAdd),
		Target:  clusterID,
		Outcome: audit.OutcomeSuccess,
		Details: map[string]any{"name": req.Name, "url": req.URL},
	})

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": req.Name, "url": req.URL})
}

// Remove deletes a repo by name for the cluster. Best-effort against helm
// (helm repo remove may fail if the repo wasn't actually added on this host —
// we still drop the DB row so the UI stays consistent).
//
// DELETE /api/clusters/{id}/apps/repos/{name}
func (h *RepoHandler) Remove(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	uid, _ := auth.UserIDFromContext(r.Context())

	if name == "" {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoRemove),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "missing_name"},
		})
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	helm, cleanup, err := h.hf.For(r.Context(), clusterID)
	if err != nil {
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoRemove),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "helm_unavailable", "error": err.Error(), "name": name},
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "helm unavailable"})
		return
	}
	defer cleanup()

	// Helm error here is non-fatal: if the repo isn't on disk locally it's
	// already "removed" as far as the operator is concerned. We still attempt
	// the DB delete so a stale row doesn't linger.
	_ = helm.RepoRemove(r.Context(), name)

	if err := h.store.DeleteRepo(r.Context(), clusterID, name); err != nil {
		if errors.Is(err, ErrNotFound) {
			_, _ = audit.Write(r.Context(), h.core, audit.Entry{
				ActorID: uid,
				Action:  string(audit.ActionAppRepoRemove),
				Target:  clusterID,
				Outcome: audit.OutcomeFailure,
				Details: map[string]any{"reason": "not_found", "name": name},
			})
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "repo not found"})
			return
		}
		_, _ = audit.Write(r.Context(), h.core, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionAppRepoRemove),
			Target:  clusterID,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "db_error", "error": err.Error(), "name": name},
		})
		writeJSON(w, http.StatusInternalServerError, errMap(err))
		return
	}

	if h.catalog != nil {
		h.catalog.Invalidate(clusterID)
	}

	_, _ = audit.Write(r.Context(), h.core, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionAppRepoRemove),
		Target:  clusterID,
		Outcome: audit.OutcomeSuccess,
		Details: map[string]any{"name": name},
	})

	w.WriteHeader(http.StatusNoContent)
}

// ---------- helpers ----------

func ptrInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func errMap(err error) map[string]string {
	return map[string]string{"error": err.Error()}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
