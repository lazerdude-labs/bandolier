package clusters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// UpgradeExecutor is the minimal subset of *deployments.Executor the upgrade
// handler depends on, kept narrow so tests can substitute a fake.
type UpgradeExecutor interface {
	Upgrade(ctx context.Context, clusterID, k3sVersion string) (string, error)
}

type upgradeHandler struct {
	store *store.Store
	exec  UpgradeExecutor
}

// NewUpgradeHandler returns an http.Handler for POST /api/clusters/{id}/upgrade.
func NewUpgradeHandler(s *store.Store, e UpgradeExecutor) http.Handler {
	return &upgradeHandler{store: s, exec: e}
}

type upgradeReq struct {
	K3sVersion string `json:"k3s_version"`
}

func (h *upgradeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body upgradeReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	v := strings.TrimSpace(body.K3sVersion)
	// Loose semver guard: a real k3s release tag always contains "+k3s" — eg
	// "v1.31.12+k3s1". Cheap defense against typos before we kick off ansible.
	if v == "" || !strings.Contains(v, "+k3s") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "k3s_version must look like 'v1.31.12+k3s1'"})
		return
	}
	depID, err := h.exec.Upgrade(r.Context(), id, v)
	switch {
	case errors.Is(err, ErrInvalidTransition):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "cluster cannot be upgraded in current state"})
	case errors.Is(err, ErrLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "deployment in progress"})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "upgrade failed"})
	default:
		writeJSON(w, http.StatusAccepted, map[string]string{"deployment_id": depID})
	}
}
