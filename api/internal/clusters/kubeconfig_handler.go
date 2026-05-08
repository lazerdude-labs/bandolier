package clusters

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/auth"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

type kubeconfigHandler struct {
	store *store.Store
	vault *vault.Client
}

func NewKubeconfigHandler(s *store.Store, v *vault.Client) http.Handler {
	return &kubeconfigHandler{store: s, vault: v}
}

func (h *kubeconfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	uid, _ := auth.UserIDFromContext(r.Context())
	c, err := h.store.GetCluster(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterKubeconfigDownload),
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

	paths := vault.Paths{}
	data, err := h.vault.Get(r.Context(), paths.Kubeconfig(id))
	if err != nil || data == nil {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterKubeconfigDownload),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "kubeconfig_missing"},
		})
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "kubeconfig not yet retrieved"})
		return
	}
	yaml, _ := data["yaml"].(string)
	if yaml == "" {
		_, _ = audit.Write(r.Context(), h.store, audit.Entry{
			ActorID: uid,
			Action:  string(audit.ActionClusterKubeconfigDownload),
			Target:  id,
			Outcome: audit.OutcomeFailure,
			Details: map[string]any{"reason": "kubeconfig_missing"},
		})
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "kubeconfig empty"})
		return
	}

	_, _ = audit.Write(r.Context(), h.store, audit.Entry{
		ActorID: uid,
		Action:  string(audit.ActionClusterKubeconfigDownload),
		Target:  id,
		Outcome: audit.OutcomeSuccess,
	})
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, c.Name))
	_, _ = w.Write([]byte(yaml))
}

// RetrieveExecutor decouples the handler from the deployments package so it
// can be unit-tested without spinning up the executor.
type RetrieveExecutor interface {
	RetrieveKubeconfig(ctx context.Context, clusterID string) error
}

type kubeconfigRetrieveHandler struct {
	store *store.Store
	exec  RetrieveExecutor
}

func NewKubeconfigRetrieveHandler(s *store.Store, e RetrieveExecutor) http.Handler {
	return &kubeconfigRetrieveHandler{store: s, exec: e}
}

func (h *kubeconfigRetrieveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.exec.RetrieveKubeconfig(r.Context(), id)
	switch {
	case errors.Is(err, ErrClusterNotReady):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
