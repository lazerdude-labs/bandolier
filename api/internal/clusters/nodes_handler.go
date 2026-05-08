package clusters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
	"github.com/lazerdude-labs/bandolier/api/internal/telemetry"
)

type NodeAggregator interface {
	NodeTelemetry(ctx context.Context, clusterID string) ([]telemetry.NodeTelemetry, error)
}

type nodesHandler struct {
	store *store.Store
	agg   NodeAggregator
}

func NewNodesHandler(s *store.Store, agg NodeAggregator) http.Handler {
	return &nodesHandler{store: s, agg: agg}
}

func (h *nodesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.store.GetCluster(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "cluster not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rows, err := h.agg.NodeTelemetry(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rows == nil {
		rows = []telemetry.NodeTelemetry{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}
