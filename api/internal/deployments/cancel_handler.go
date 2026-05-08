package deployments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// CancelExecutor is the narrow surface the cancel handler depends on, kept
// small so tests can substitute a fake without standing up the full Executor.
type CancelExecutor interface {
	Cancel(ctx context.Context, depID string) error
}

type cancelHandler struct {
	store *store.Store
	exec  CancelExecutor
}

// NewCancelHandler returns an http.Handler for POST /api/deployments/{id}/cancel.
// Responds:
//
//	202 + deployment row on successful cancel signal (terminal write happens
//	    asynchronously inside the goroutine when its ctx unwinds);
//	404 when the deployment id is unknown;
//	409 when the deployment is not in 'running' state, or when the executor
//	    has no live cancel func registered (already finished or never started).
func NewCancelHandler(s *store.Store, e CancelExecutor) http.Handler {
	return &cancelHandler{store: s, exec: e}
}

func (h *cancelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	dep, err := h.store.GetDeployment(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if dep.Status != "running" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "deployment is not running"})
		return
	}

	if err := h.exec.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotCancellable) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "deployment is not cancellable"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, dep)
}

// writeJSON is a tiny local helper — the deployments package previously got
// by without one because the existing handlers inline json.NewEncoder. Kept
// private here to match the rest of the codebase's per-package convention.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
